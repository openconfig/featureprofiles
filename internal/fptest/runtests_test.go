package fptest

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	mpb "github.com/openconfig/featureprofiles/proto/metadata_go_proto"
	ripb "github.com/openconfig/featureprofiles/proto/release_intent_go_proto"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ygnmi/ygnmi"
	"google.golang.org/protobuf/proto"
)

// TestDatapointValidator confirms behavior of datapointValidator.
func TestDatapointValidator(t *testing.T) {
	tests := []struct {
		name    string
		dp      *ygnmi.DataPoint
		wantErr bool
	}{
		{
			name: "valid timestamp",
			dp: &ygnmi.DataPoint{
				Timestamp:     time.Unix(1707215426, 123456789),
				RecvTimestamp: time.Unix(1707215426, 123456790),
			},
			wantErr: false,
		},
		{
			name: "receive timestamp before notification timestamp",
			dp: &ygnmi.DataPoint{
				Timestamp:     time.Unix(1707215426, 123456789),
				RecvTimestamp: time.Unix(1707215426, 123456788),
			},
			wantErr: true,
		},
		{
			name: "zero timestamp",
			dp: &ygnmi.DataPoint{
				Timestamp:     time.Time{},
				RecvTimestamp: time.Unix(1707215426, 123456790),
			},
			wantErr: false,
		},
		{
			name: "valid UTF-8 string",
			dp: &ygnmi.DataPoint{
				Timestamp:     time.Unix(1707215426, 123456789),
				RecvTimestamp: time.Unix(1707215426, 123456790),
				Value:         &gpb.TypedValue{Value: &gpb.TypedValue_StringVal{StringVal: "hello"}},
			},
			wantErr: false,
		},
		{
			name: "invalid UTF-8 string",
			dp: &ygnmi.DataPoint{
				Timestamp:     time.Unix(1707215426, 123456789),
				RecvTimestamp: time.Unix(1707215426, 123456790),
				Value:         &gpb.TypedValue{Value: &gpb.TypedValue_StringVal{StringVal: "\xff\xfe\xfd"}},
			},
			wantErr: true,
		},
		{
			name: "empty string",
			dp: &ygnmi.DataPoint{
				Timestamp:     time.Unix(1707215426, 123456789),
				RecvTimestamp: time.Unix(1707215426, 123456790),
				Value:         &gpb.TypedValue{Value: &gpb.TypedValue_StringVal{StringVal: ""}},
			},
			wantErr: false,
		},
		{
			name: "non-string value",
			dp: &ygnmi.DataPoint{
				Timestamp:     time.Unix(1707215426, 123456789),
				RecvTimestamp: time.Unix(1707215426, 123456790),
				Value:         &gpb.TypedValue{Value: &gpb.TypedValue_IntVal{IntVal: 123}},
			},
			wantErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := datapointValidator(tc.dp)
			if (err != nil) != tc.wantErr {
				t.Errorf("datapointValidator(%v) error = %v, wantErr %v", tc.dp, err, tc.wantErr)
			}
		})
	}
}

func TestIsTestIntended(t *testing.T) {
	tmpDir := t.TempDir()

	textprotoPath := filepath.Join(tmpDir, "release_intent.textproto")
	if err := os.WriteFile(textprotoPath, []byte(`
release_id: "2026.1"
intended_test_ids: "ACL-1.2"
intended_test_ids: "gNOI-4.1"
`), 0644); err != nil {
		t.Fatalf("Failed to write temporary textproto intent file: %v", err)
	}

	binaryProtoPath := filepath.Join(tmpDir, "release_intent.pb")
	relIntent := &ripb.ReleaseIntent{
		ReleaseId:       "2026.1",
		IntendedTestIds: []string{"TE-1.21", "TE-3.3"},
	}
	binaryData, err := proto.Marshal(relIntent)
	if err != nil {
		t.Fatalf("Failed to marshal binary ReleaseIntent: %v", err)
	}
	if err := os.WriteFile(binaryProtoPath, binaryData, 0644); err != nil {
		t.Fatalf("Failed to write temporary binary intent file: %v", err)
	}

	invalidProtoPath := filepath.Join(tmpDir, "invalid.textproto")
	if err := os.WriteFile(invalidProtoPath, []byte("invalid ::: proto ::: content"), 0644); err != nil {
		t.Fatalf("Failed to write invalid intent file: %v", err)
	}

	tests := []struct {
		name       string
		intentPath string
		md         *mpb.Metadata
		want       bool
		wantErr    bool
	}{
		{
			name:       "empty intent allows all tests",
			intentPath: "",
			md:         &mpb.Metadata{PlanId: "ACL-1.2"},
			want:       true,
			wantErr:    false,
		},
		{
			name:       "test plan ID in textproto intent",
			intentPath: textprotoPath,
			md:         &mpb.Metadata{PlanId: "ACL-1.2"},
			want:       true,
			wantErr:    false,
		},
		{
			name:       "second test plan ID in textproto intent",
			intentPath: textprotoPath,
			md:         &mpb.Metadata{PlanId: "gNOI-4.1"},
			want:       true,
			wantErr:    false,
		},
		{
			name:       "test plan ID not in textproto intent",
			intentPath: textprotoPath,
			md:         &mpb.Metadata{PlanId: "TE-3.8"},
			want:       false,
			wantErr:    false,
		},
		{
			name:       "test plan ID in binary proto intent",
			intentPath: binaryProtoPath,
			md:         &mpb.Metadata{PlanId: "TE-1.21"},
			want:       true,
			wantErr:    false,
		},
		{
			name:       "nil metadata with intent set is not intended",
			intentPath: textprotoPath,
			md:         nil,
			want:       false,
			wantErr:    false,
		},
		{
			name:       "empty plan ID with intent set is not intended",
			intentPath: textprotoPath,
			md:         &mpb.Metadata{PlanId: ""},
			want:       false,
			wantErr:    false,
		},
		{
			name:       "nonexistent intent file returns error",
			intentPath: filepath.Join(tmpDir, "nonexistent.textproto"),
			md:         &mpb.Metadata{PlanId: "ACL-1.2"},
			want:       false,
			wantErr:    true,
		},
		{
			name:       "invalid intent file content returns error",
			intentPath: invalidProtoPath,
			md:         &mpb.Metadata{PlanId: "ACL-1.2"},
			want:       false,
			wantErr:    true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := isTestIntended(tc.intentPath, tc.md)
			if (err != nil) != tc.wantErr {
				t.Fatalf("isTestIntended(%q, %v) error = %v, wantErr %v", tc.intentPath, tc.md, err, tc.wantErr)
			}
			if got != tc.want {
				t.Errorf("isTestIntended(%q, %v) = %v, want %v", tc.intentPath, tc.md, got, tc.want)
			}
		})
	}
}
