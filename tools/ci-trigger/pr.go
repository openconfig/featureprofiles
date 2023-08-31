// Copyright 2023 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"github.com/google/go-github/v50/github"
	"github.com/google/uuid"
	"google.golang.org/api/cloudbuild/v1"

	"github.com/go-git/go-git/v5"
	"github.com/golang/glog"
	"google.golang.org/protobuf/encoding/prototext"

	mpb "github.com/openconfig/featureprofiles/proto/metadata_go_proto"
	opb "github.com/openconfig/ondatra/proto"
)

type pullRequest struct {
	ID       int
	HeadSHA  string
	Virtual  []device
	Physical []device

	cloneURL string

	repo      *git.Repository
	localFS   fs.FS
	localPath string
}

type device struct {
	Type                deviceType
	CloudBuildID        string
	CloudBuildLogURL    string
	CloudBuildRawLogURL string
	Tests               []functionalTest
}

type deviceType struct {
	Vendor        opb.Device_Vendor
	HardwareModel string
}

type functionalTest struct {
	Name        string
	Description string
	Path        string
	DocURL      string
	TestURL     string
	Status      string
	BadgePath   string
	BadgeURL    string
}

func (d *deviceType) String() string {
	return d.Vendor.String() + " " + d.HardwareModel
}

// createArchive uploads the compressed repository to Object Store and returns the path to the object.
func (p *pullRequest) createArchive(ctx context.Context, storClient *storage.Client) (string, error) {
	data, err := createTGZArchive(p.localFS)
	if err != nil {
		return "", err
	}

	u, err := uuid.NewRandom()
	if err != nil {
		return "", err
	}

	objPath := "source/" + strconv.FormatInt(time.Now().UTC().Unix(), 10) + "-" + hex.EncodeToString(u[:]) + ".tgz"
	obj := storClient.Bucket(gcpCloudBuildBucketName).Object(objPath).NewWriter(ctx)
	obj.ContentType = "application/x-tar"
	obj.Metadata = map[string]string{
		"pr":      strconv.Itoa(p.ID),
		"headSHA": p.HeadSHA,
	}
	if _, err := data.WriteTo(obj); err != nil {
		return "", err
	}
	return objPath, obj.Close()
}

// createBuild creates a GCB build for each of the deviceTypes.
func (p *pullRequest) createBuild(ctx context.Context, buildClient *cloudbuild.Service, storClient *storage.Client, devices []deviceType) error {
	err := p.fetchGoDeps()
	if err != nil {
		return err
	}

	objPath, err := p.createArchive(ctx, storClient)
	if err != nil {
		return err
	}

	for _, d := range devices {
	virtualDeviceLoop:
		for i, virtualDevice := range p.Virtual {
			if virtualDevice.Type == d {
				if len(virtualDevice.Tests) == 0 {
					continue
				}
				for _, v := range virtualDevice.Tests {
					if v.Status != "pending authorization" {
						continue virtualDeviceLoop
					}
				}
				cb := &cloudBuild{
					device:      virtualDevice,
					buildClient: buildClient,
					storClient:  storClient,
					f:           p.localFS,
				}
				jobID, logURL, err := cb.submitBuild(objPath)
				if err != nil {
					return fmt.Errorf("submitBuild device %q: %w", virtualDevice.Type.String(), err)
				}
				glog.Infof("Created CloudBuild Job %s for PR%d at commit %q for device %q", jobID, p.ID, p.HeadSHA, virtualDevice.Type.String())
				p.Virtual[i].CloudBuildID = jobID
				p.Virtual[i].CloudBuildLogURL = logURL
				vendor := strings.ToLower(virtualDevice.Type.Vendor.String())
				vendor = strings.ReplaceAll(vendor, " ", "")
				p.Virtual[i].CloudBuildRawLogURL = fmt.Sprintf("https://storage.cloud.google.com/featureprofiles-ci-logs-%s/log-%s.txt", vendor, jobID)
				for j := range virtualDevice.Tests {
					p.Virtual[i].Tests[j].Status = "setup"
				}
			}
		}
	}

	return nil
}

// fetchGoDeps downloads the Golang module dependencies into a local vendor cache.
func (p *pullRequest) fetchGoDeps() error {
	goBin, err := exec.LookPath("go")
	if err != nil {
		return err
	}
	cmd := exec.Command(goBin, "mod", "vendor")
	cmd.Dir = p.localPath
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("vendoring dependencies: %v, output:\n%s", err, out)
	}
	return nil
}

// identifyModifiedTests gathers all of the tests that have been modified in the pull request.
func (p *pullRequest) identifyModifiedTests() error {
	if p.repo == nil {
		var err error
		p.repo, err = setupGitClone(p.localPath, p.cloneURL, p.HeadSHA)
		if err != nil {
			return err
		}
	}

	ft, err := functionalTestPaths(p.localFS)
	if err != nil {
		return err
	}

	mf, err := modifiedFiles(p.repo, p.HeadSHA)
	if err != nil {
		return err
	}
	modifiedTests := modifiedFunctionalTests(ft, mf)

	return p.populateTestDetail(modifiedTests)
}

// populateObjectMetadata gathers the metadata from Object Store for any tests that exist.
func (p *pullRequest) populateObjectMetadata(ctx context.Context, storClient *storage.Client) {
	for i, virtualDevice := range p.Virtual {
		for j, test := range virtualDevice.Tests {
			objAttrs, err := storClient.Bucket(gcpBucket).Object(test.BadgePath).Attrs(ctx)
			if err != nil {
				glog.Infof("Failed to fetch object %s attrs: %s", test.BadgePath, err)
				continue
			}
			if status, ok := objAttrs.Metadata["status"]; ok {
				p.Virtual[i].Tests[j].Status = status
			}
			if cloudBuildID, ok := objAttrs.Metadata["cloudBuild"]; ok {
				p.Virtual[i].CloudBuildID = cloudBuildID
			}
			if cloudBuildLogURL, ok := objAttrs.Metadata["cloudBuildLogURL"]; ok {
				p.Virtual[i].CloudBuildLogURL = cloudBuildLogURL
			}
			if cloudBuildRawLogURL, ok := objAttrs.Metadata["cloudBuildRawLogURL"]; ok {
				p.Virtual[i].CloudBuildRawLogURL = cloudBuildRawLogURL
			}
		}
	}
}

// updateBadges creates or updates the status of all badges in Google
// Cloud Storage to reflect the current status of the pullRequest.
func (p *pullRequest) updateBadges(ctx context.Context, storClient *storage.Client) error {
	var allDevices []device
	allDevices = append(allDevices, p.Physical...)
	allDevices = append(allDevices, p.Virtual...)
	for _, device := range allDevices {
		for _, test := range device.Tests {
			buf, err := svgBadge(test.Name, test.Status)
			if err != nil {
				return err
			}
			obj := storClient.Bucket(gcpBucket).Object(test.BadgePath).NewWriter(ctx)
			obj.ContentType = "image/svg+xml"
			obj.CacheControl = "no-cache,max-age=0"
			obj.Metadata = map[string]string{
				"status":              test.Status,
				"label":               test.Name,
				"cloudBuild":          device.CloudBuildID,
				"cloudBuildLogURL":    device.CloudBuildLogURL,
				"cloudBuildRawLogURL": device.CloudBuildRawLogURL,
			}
			if _, err := buf.WriteTo(obj); err != nil {
				return err
			}
			if err := obj.Close(); err != nil {
				return err
			}
		}
	}

	return nil
}

// updateGitHub adds or updates a comment to the GitHub pull request with the
// current status of all tests.
func (p *pullRequest) updateGitHub(ctx context.Context, githubClient *github.Client) error {
	var buf bytes.Buffer

	if err := commentTpl.Execute(&buf, p); err != nil {
		return err
	}
	comment := &github.IssueComment{
		Body: github.String(buf.String()),
	}

	firstComment, err := p.firstComment(ctx, githubClient)
	if err != nil {
		return err
	}
	if firstComment == nil {
		err = withRetry(3, "CreateIssueComment", func() error {
			_, _, err = githubClient.Issues.CreateComment(ctx, githubProjectOwner, githubProjectRepo, p.ID, comment)
			return err
		})
	} else {
		err = withRetry(3, "EditIssueComment", func() error {
			_, _, err = githubClient.Issues.EditComment(ctx, githubProjectOwner, githubProjectRepo, firstComment.GetID(), comment)
			return err
		})
	}
	return err
}

// populateTestDetail reads the metadata.textproto from each test in
// functionalTests and populates the pullRequest with test details.
func (p *pullRequest) populateTestDetail(functionalTests []string) error {
	tests := make(map[deviceType][]functionalTest)
	for _, ft := range functionalTests {
		in, err := fs.ReadFile(p.localFS, ft+"/metadata.textproto")
		if err != nil {
			return err
		}
		md := &mpb.Metadata{}
		if err := (prototext.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(in, md); err != nil {
			return err
		}
		for _, d := range virtualDeviceTypes {
			// We can't support virtual ATE tests - skip them.
			if strings.Contains(ft, "ate_tests") {
				continue
			}
			badgeTestName := base64.RawURLEncoding.EncodeToString([]byte(ft))
			deviceName := strings.ReplaceAll(d.String(), " ", "_")
			badgePath := gcpBucketPrefix + "/" + strconv.Itoa(p.ID) + "/" + p.HeadSHA + "/" + badgeTestName + "." + deviceName + ".svg"
			badgeURL := "https://storage.googleapis.com/" + gcpBucket + "/" + badgePath
			newTest := functionalTest{
				Name:        md.PlanId,
				Description: md.Description,
				Path:        ft,
				DocURL:      "https://github.com/" + githubProjectOwner + "/" + githubProjectRepo + "/blob/" + p.HeadSHA + "/" + ft + "/README.md",
				TestURL:     "https://github.com/" + githubProjectOwner + "/" + githubProjectRepo + "/blob/" + p.HeadSHA + "/" + ft,
				BadgeURL:    badgeURL,
				BadgePath:   badgePath,
				Status:      "pending authorization",
			}
			tests[d] = append(tests[d], newTest)
		}
	}

	for _, d := range virtualDeviceTypes {
		if dt, ok := tests[d]; ok {
			p.Virtual = append(p.Virtual, device{
				Type:  d,
				Tests: dt,
			})
		}
	}

	return nil
}

// firstComment returns the first bot-originated comment in the PR if it exists.
func (p *pullRequest) firstComment(ctx context.Context, githubClient *github.Client) (*github.IssueComment, error) {
	comments, _, err := githubClient.Issues.ListComments(ctx, githubProjectOwner, githubProjectRepo, p.ID, nil)
	if err != nil {
		return nil, err
	}

	for _, comment := range comments {
		if comment.GetUser().GetLogin() == githubBotName {
			return comment, nil
		}
	}

	return nil, nil
}

// functionalTestPaths returns a list of directories containing functional test metadata.
func functionalTestPaths(f fs.FS) ([]string, error) {
	var testDirs []string
	err := fs.WalkDir(f, "feature", func(path string, d fs.DirEntry, err error) error {
		if !d.IsDir() && d.Name() == "metadata.textproto" {
			testDirs = append(testDirs, filepath.Dir(path))
		}
		return nil
	})
	return testDirs, err
}

// modifiedFunctionalTests checks if any values of modifiedFiles starts with
// functionalTests.  The list returned contains functional test paths with at
// least one modified file.
func modifiedFunctionalTests(functionalTests []string, modifiedFiles []string) []string {
	fts := make(map[string]struct{})
	for _, ft := range functionalTests {
		for _, mf := range modifiedFiles {
			if strings.HasPrefix(mf, ft) {
				fts[ft] = struct{}{}
			}
		}
	}
	var result []string
	for k := range fts {
		result = append(result, k)
	}
	return result
}

// withRetry will run func f up to attempts times, retrying if any error is
// returned. This is intended to be used with the GitHub HTTP API, which can
// occasionally return errors that deserve a retry.
func withRetry(attempts int, name string, f func() error) error {
	var err error
	for i := 0; i < attempts; i++ {
		if err = f(); err == nil {
			return nil
		}
		glog.Infof("Retry %d of %q, error: %v", attempts, name, err)
		time.Sleep(250 * time.Millisecond)
	}
	return err
}
