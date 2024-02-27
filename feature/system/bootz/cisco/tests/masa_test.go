package bootz

import (
	"context"
	"crypto/rand"
	"fmt"
	"log"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/openconfig/ovgs"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestMasa(t *testing.T) {
	ovsC, ctx := generateVoucherClient(t)
	t.Run("Get Group : verify group ID, group Description and child ids ", func(t *testing.T) {
		getGroupReq, err := ovsC.GetGroup(ctx, &ovgs.GetGroupRequest{})
		t.Logf("Get Group req : %v", prettyPrint(getGroupReq))
		if err != nil {
			t.Errorf("Error while doing Get Group %v ", err)
		}
		if getGroupReq.GetGroupId() != *groupID {
			t.Errorf("Group ID not returned as expected, want : %v, got %v", groupID, getGroupReq.GetGroupId())
		}
		if getGroupReq.GetDescription() != *groupDes {
			t.Errorf("Group description does not match want :%v , got: %v", groupDes, getGroupReq.GetDescription())
		}
		if len(getGroupReq.GetChildGroupIds()) > 0 {
			t.Errorf("Child groups expected to be empty want:%v, got %v", 0, len(getGroupReq.GetChildGroupIds()))
		}

	})
	t.Run("Create and Delete group ", func(t *testing.T) {
		createRes, err := ovsC.CreateGroup(ctx, &ovgs.CreateGroupRequest{Parent: *groupID, Description: "Child "})
		if err == nil {
			t.Errorf("Error expected while creating a group %v ", err)
		}
		t.Logf("Create response %v ", createRes.GetGroupId())
		delRes, err := ovsC.DeleteGroup(ctx, &ovgs.DeleteGroupRequest{GroupId: *groupID})
		if err == nil {
			t.Errorf("Error expected while creating a group %v ", err)
		}
		t.Logf("Delete response %v ", delRes.String())
	})
	t.Run("Testing Valid User Role ", func(t *testing.T) {
		//Remove user if already exists
		deluserRole := &ovgs.RemoveUserRoleRequest{Username: username, UserType: ovgs.AccountType_ACCOUNT_TYPE_USER, OrgId: *orgID, GroupId: *groupID}
		t.Logf("Remove user Role req : %v", prettyPrint(deluserRole))
		_, _ = ovsC.RemoveUserRole(ctx, deluserRole)
		//Add user
		cases := []struct {
			username string
			userrole *ovgs.UserRole
		}{
			{
				username: username,
				userrole: ovgs.UserRole_USER_ROLE_ADMIN.Enum(),
			},
			{
				username: username,
				userrole: ovgs.UserRole_USER_ROLE_ASSIGNER.Enum(),
			},
			{
				username: username,
				userrole: ovgs.UserRole_USER_ROLE_REQUESTOR.Enum(),
			},
		}
		for _, tc := range cases {
			userRole := &ovgs.AddUserRoleRequest{Username: tc.username, UserType: ovgs.AccountType_ACCOUNT_TYPE_USER, OrgId: *orgID, GroupId: *groupID, UserRole: ovgs.UserRole(tc.userrole.Number())}
			t.Logf("Add user Role req : %v", prettyPrint(userRole))
			addResp, err := ovsC.AddUserRole(ctx, userRole)
			if err != nil {
				t.Errorf("Error while adding userRole %v ", err)
			}
			t.Logf("Add user role response %v", addResp.String())
			// Get user
			getUserRoleReq := &ovgs.GetUserRoleRequest{Username: tc.username, UserType: ovgs.AccountType_ACCOUNT_TYPE_USER, OrgId: *orgID}
			t.Logf("Get user Role req : %v", prettyPrint(getUserRoleReq))
			getUserRes, err := ovsC.GetUserRole(ctx, getUserRoleReq)
			if err != nil {
				t.Errorf("Error while doing Get userRole %v ", err)
			}
			if getUserRes.GetGroups()[tc.username].Enum().String() != tc.userrole.String() {
				t.Errorf("Get user role response not as expected, want: %v , got: %v", tc.userrole, getUserRes.GetGroups()[tc.username].Enum())
			}

			// 	//Delete UserRole
			deluserRole := &ovgs.RemoveUserRoleRequest{Username: tc.username, UserType: ovgs.AccountType_ACCOUNT_TYPE_USER, OrgId: *orgID, GroupId: *groupID}
			t.Logf("Remove user Role req : %v", prettyPrint(deluserRole))
			delResp, err := ovsC.RemoveUserRole(ctx, deluserRole)
			if err != nil {
				t.Errorf("Error while removing userRole %v ", err)
			}
			t.Logf("Remove user role response %v", delResp.String())
		}

	})
	t.Run("Testing Valid Serial Numbers", func(t *testing.T) {
		// //Remove Serial Number if exists
		removeReq := &ovgs.RemoveSerialRequest{GroupId: *groupID, Component: &ovgs.Component{SerialNumber: serialNumber}}
		t.Logf("Remove  serial number request: %v", prettyPrint(removeReq))
		removeRes, err := ovsC.RemoveSerial(ctx, removeReq)
		if (err != nil) && (!strings.Contains(err.Error(), "Serial Number not found")) {
			t.Errorf("Error while deleting Serial Number %v ", err)
		}
		t.Logf("remove serial number response %v", removeRes.String())
		//Add Serial Number
		serialNumberReq := &ovgs.AddSerialRequest{Component: &ovgs.Component{SerialNumber: serialNumber}, GroupId: *groupID}
		t.Logf("Adding  serial number request: %v", prettyPrint(serialNumberReq))
		addResp, err := ovsC.AddSerial(ctx, serialNumberReq)
		if err != nil {
			t.Errorf("Error while adding Serial Number %v ", err)
		}
		t.Logf("Add Serial Number response %v", addResp.String())

		//Get Serial Number
		getserialReq := &ovgs.GetSerialRequest{Component: &ovgs.Component{SerialNumber: serialNumber}}
		t.Logf("Get  serial number request: %v", prettyPrint(getserialReq))
		getResp, err := ovsC.GetSerial(ctx, getserialReq)
		if err != nil {
			t.Errorf("Error while getting Serial Number %v ", err)
		}
		t.Logf("Get Serial Number response %v", getResp.String())

		if getResp.GetModel() != modelName {
			t.Errorf("Model name not returned as expected , want : %v, got %v", modelName, getResp.GetModel())
		}

		t.Logf("Group IDs %v", getResp.GetGroupIds()[0])
		if getResp.GetGroupIds()[0] != *groupID {
			t.Errorf("Group ID not returned as expected, want : %v, got %v", groupID, getResp.GetGroupIds()[0])
		}
		//Remove the serial number successfully
		removeReq = &ovgs.RemoveSerialRequest{GroupId: *groupID, Component: &ovgs.Component{SerialNumber: serialNumber}}
		t.Logf("Remove  serial number request: %v", prettyPrint(removeReq))
		removeRes, err = ovsC.RemoveSerial(ctx, removeReq)
		t.Logf("remove serial number response %v", removeRes.String())
		if err != nil {
			t.Errorf("Error while deleting Serial Number %v ", err)
		}

	})

	t.Run("Domain Cert and Voucher Generation", func(t *testing.T) {
		//Add Serial Number
		serialNumberReq := &ovgs.AddSerialRequest{Component: &ovgs.Component{SerialNumber: serialNumber}, GroupId: *groupID}
		t.Logf("Adding  serial number request: %v", prettyPrint(serialNumberReq))
		addResp, err := ovsC.AddSerial(ctx, serialNumberReq)
		if (err != nil) && (!strings.Contains(err.Error(), "This serial number has already been added to this organization")) {
			t.Errorf("Error while adding Serial Number %v ", err)
		}
		t.Logf("Add Serial Number response %v", addResp.String())

		//Create Domain Cert
		expiryDate := timestamppb.New(time.Date(2024, 12, 1, 1, 1, 1, 1, time.UTC))
		domainCertReq := &ovgs.CreateDomainCertRequest{
			GroupId:        *groupID,
			CertificateDer: readPDCreturnParsed(pdcFile),
			ExpiryTime:     expiryDate,
		}
		t.Logf("Creating Domain Cert : %s\n", prettyPrint(domainCertReq.GetCertificateDer()))

		domainCertresp, err := ovsC.CreateDomainCert(ctx, domainCertReq)
		if err != nil {
			t.Errorf("Create Domain Cert failed: %v \n", err)
		}
		//Get Domain Cert
		domainCertGetReq := &ovgs.GetDomainCertRequest{CertId: domainCertresp.GetCertId()}
		domainCertGetRes, err := ovsC.GetDomainCert(ctx, domainCertGetReq)
		if err != nil {
			t.Errorf("Getting Domain Cert failed: %v \n", err)
		}
		t.Logf("Domain Cert response %v", domainCertGetRes)

		if domainCertGetRes.GetCertId() != domainCertresp.GetCertId() {
			t.Errorf("Cert ID returned by Get Domain Cert is not valid, want: %v, got: %v", domainCertresp.GetCertId(), domainCertGetRes.GetCertId())
		}
		if domainCertGetRes.GetExpiryTime().Seconds != expiryDate.Seconds {
			t.Errorf("Expiry Date returned by Get Domain Cert is not valid, want: %v, got: %v", expiryDate, domainCertGetRes.GetExpiryTime())
		}
		if domainCertGetRes.GetCertificateDer() == nil {
			t.Errorf("Do not expect Certificate Der to be nil")
		}
		if domainCertGetRes.GetGroupId() != *groupID {
			t.Errorf("Group ID returned by Get Domain Cert is not valid, want: %v, got: %v", groupID, domainCertGetRes.GetGroupId())
		}

		if domainCertGetRes.GetRevocationChecks() != false {
			t.Errorf("Do not expect Revocation Checks to be False")
		}

		reqOV := &ovgs.GetOwnershipVoucherRequest{
			Component: &ovgs.Component{SerialNumber: serialNumber},
			CertId:    domainCertresp.CertId,
			Lifetime:  timestamppb.New(time.Date(2024, 12, 1, 1, 1, 1, 1, time.UTC)),
		}
		t.Logf("Getting OV serial number request: %v", prettyPrint(reqOV))
		voucherGot, err := ovsC.GetOwnershipVoucher(ctx, reqOV)
		if err != nil {
			fmt.Printf("Getting Voucher is failed %v \n", err)
		}

		t.Logf("Voucher in CMS format %v", string(voucherGot.GetVoucherCms()))
		err = os.WriteFile(voucherGenerated, voucherGot.GetVoucherCms(), 0600)
		if err != nil {
			t.Errorf("Err while writing voucher to the file %v", err)
		}
		//Delete Domain Cert
		domainCertDelReq := &ovgs.DeleteDomainCertRequest{CertId: domainCertresp.GetCertId()}
		domainCertDelRes, err := ovsC.DeleteDomainCert(ctx, domainCertDelReq)
		if err != nil {
			t.Errorf("Error while deleting Domain Cert %v", err)
		}
		t.Logf("Delete Domain Cert response %v ", domainCertDelRes.String())
	})

	t.Run("Get on invalid group ID ", func(t *testing.T) {
		getGroupRes, err := ovsC.GetGroup(ctx, &ovgs.GetGroupRequest{GroupId: "XYZ"})
		t.Logf("Get group RPC : %v", prettyPrint(&ovgs.GetGroupRequest{GroupId: "XYZ"}))
		if err != nil {
			t.Errorf("Error while doing get on invalid group ID")
		}
		t.Logf("Get group Response %v ", getGroupRes)

	})

	t.Run("Voucher with invalid expiry date", func(t *testing.T) {
		//Add Serial Number
		serialNumberReq := &ovgs.AddSerialRequest{Component: &ovgs.Component{SerialNumber: serialNumber}, GroupId: *groupID}
		t.Logf("Adding  serial number request: %v", prettyPrint(serialNumberReq))
		addResp, err := ovsC.AddSerial(ctx, serialNumberReq)
		if (err != nil) && (!strings.Contains(err.Error(), "This serial number has already been added to this organization")) {
			t.Errorf("Error while adding Serial Number %v ", err)
		}
		t.Logf("Add Serial Number response %v", addResp.String())

		//Create Domain Cert with invalid expiry Date
		expiryDate := timestamppb.New(time.Date(2012, 12, 1, 1, 1, 1, 1, time.UTC))
		domainCertReq := &ovgs.CreateDomainCertRequest{
			GroupId:        *groupID,
			CertificateDer: readPDCreturnParsed(pdcFile),
			ExpiryTime:     expiryDate,
		}
		t.Logf("Creating Domain Cert : %s\n", prettyPrint(domainCertReq.GetCertificateDer()))

		domainCertresp, err := ovsC.CreateDomainCert(ctx, domainCertReq)
		if err != nil {
			t.Errorf("Create Domain Cert failed: %v \n", err)
		}
		//Get Domain Cert
		domainCertGetReq := &ovgs.GetDomainCertRequest{CertId: domainCertresp.GetCertId()}
		domainCertGetRes, err := ovsC.GetDomainCert(ctx, domainCertGetReq)
		if err != nil {
			t.Errorf("Err while creating domain cert %v", err)
		}
		t.Logf("Domain cert response Time : %v ", domainCertGetRes.GetExpiryTime().AsTime())
		reqOV := &ovgs.GetOwnershipVoucherRequest{
			Component: &ovgs.Component{SerialNumber: serialNumber},
			CertId:    domainCertresp.CertId,
			Lifetime:  expiryDate,
		}
		t.Logf("Getting OV serial number request: %v", prettyPrint(reqOV))
		_, err = ovsC.GetOwnershipVoucher(ctx, reqOV)
		if err == nil {
			fmt.Printf("Getting Voucher expected to failed %v \n", err)
		}

	})

	t.Run("Create Voucher with invalid cert id", func(t *testing.T) {
		//Add Serial Number
		serialNumberReq := &ovgs.AddSerialRequest{Component: &ovgs.Component{SerialNumber: serialNumber}, GroupId: *groupID}
		t.Logf("Adding  serial number request: %v", prettyPrint(serialNumberReq))
		addResp, err := ovsC.AddSerial(ctx, serialNumberReq)
		if (err != nil) && (!strings.Contains(err.Error(), "This serial number has already been added to this organization")) {
			t.Errorf("Error while adding Serial Number %v ", err)
		}
		t.Logf("Add Serial Number response %v", addResp.String())

		//Create Domain Cert with invalid certID
		expiryDate := timestamppb.New(time.Date(2024, 12, 1, 1, 1, 1, 1, time.UTC))
		domainCertReq := &ovgs.CreateDomainCertRequest{
			GroupId:        *groupID,
			CertificateDer: readPDCreturnParsed(pdcFile),
			ExpiryTime:     expiryDate,
		}
		t.Logf("Creating Domain Cert : %v\n", domainCertReq.GetCertificateDer())
		t.Logf("Creating Domain Cert : %s\n", prettyPrint(domainCertReq.GetCertificateDer()))

		domainCertresp, err := ovsC.CreateDomainCert(ctx, domainCertReq)
		if err != nil {
			t.Errorf("Create Domain Cert failed: %v \n", err)
		}
		t.Logf("Create domain cert response %v", domainCertresp)
		//Get Domain Cert
		domainCertGetReq := &ovgs.GetDomainCertRequest{CertId: "XYZ"}
		_, err = ovsC.GetDomainCert(ctx, domainCertGetReq)
		if err == nil {
			t.Errorf("Expected error while creating domain cert %v", err)
		}

	})
	t.Run("Add Invalid Serial Numbers", func(t *testing.T) {
		//Add Serial Number
		serialNumberReq := &ovgs.AddSerialRequest{Component: &ovgs.Component{SerialNumber: "XYZ"}, GroupId: *groupID}
		_, err := ovsC.AddSerial(ctx, serialNumberReq)
		if err == nil {
			t.Errorf("Expected Error while adding Serial Number %v ", err)
		}
	})

	t.Run("Create domain cert with invalid PDC", func(t *testing.T) {
		// Specify the number of bytes you want
		numBytes := 10

		// Create a byte slice to store the random bytes
		randomBytes := make([]byte, numBytes)

		// Use crypto/rand to fill the byte slice with random data
		_, err := rand.Read(randomBytes)
		if err != nil {
			fmt.Println("Error generating random bytes:", err)
		}

		//Create Domain Cert with invalid PDC
		expiryDate := timestamppb.New(time.Date(2024, 12, 1, 1, 1, 1, 1, time.UTC))
		domainCertReq := &ovgs.CreateDomainCertRequest{
			GroupId:        *groupID,
			CertificateDer: randomBytes,
			ExpiryTime:     expiryDate,
		}
		t.Logf("Creating Domain Cert : %v\n", domainCertReq.GetCertificateDer())
		t.Logf("Creating Domain Cert : %s\n", prettyPrint(domainCertReq.GetCertificateDer()))
		_, err = ovsC.CreateDomainCert(ctx, domainCertReq)
		if err == nil {
			t.Errorf("Expected Create Domain Cert to fail: %v \n", err)
		}

	})
	t.Run("Create domain cert with revocation check true", func(t *testing.T) {
		//Add Serial Number
		serialNumberReq := &ovgs.AddSerialRequest{Component: &ovgs.Component{SerialNumber: serialNumber}, GroupId: *groupID}
		t.Logf("Adding  serial number request: %v", prettyPrint(serialNumberReq))
		addResp, err := ovsC.AddSerial(ctx, serialNumberReq)
		if (err != nil) && (!strings.Contains(err.Error(), "This serial number has already been added to this organization")) {
			t.Errorf("Error while adding Serial Number %v ", err)
		}
		t.Logf("Add Serial Number response %v", addResp.String())

		//Create Domain Cert with revocation checks true
		expiryDate := timestamppb.New(time.Date(2034, 12, 1, 1, 1, 1, 1, time.UTC))
		domainCertReq := &ovgs.CreateDomainCertRequest{
			GroupId:          *groupID,
			CertificateDer:   readPDCreturnParsed(pdcFile),
			ExpiryTime:       expiryDate,
			RevocationChecks: true,
		}
		t.Logf("Creating Domain Cert : %s\n", prettyPrint(domainCertReq.GetCertificateDer()))
		domainCertRes, err := ovsC.CreateDomainCert(ctx, domainCertReq)
		if err != nil {
			t.Errorf("Error while Create Domain Cert: %v \n", err)
		}

		reqOV := &ovgs.GetOwnershipVoucherRequest{
			Component: &ovgs.Component{SerialNumber: serialNumber},
			CertId:    domainCertRes.GetCertId(),
			Lifetime:  expiryDate,
		}
		t.Logf("Getting OV serial number request: %v", prettyPrint(reqOV))
		voucherGot, err := ovsC.GetOwnershipVoucher(ctx, reqOV)
		if err != nil {
			fmt.Printf("Getting Voucher is failed %v \n", err)
		}
		t.Logf("Voucher in CMS format %v", voucherGot.GetVoucherCms())
		err = os.WriteFile(voucherGenerated, voucherGot.GetVoucherCms(), 0600)
		if err != nil {
			t.Errorf("Err writing voucher to the file  %v", err)
		}

	})
	t.Run("Invalid User to generate Voucher", func(t *testing.T) {
		//Delete UserRole
		deluserRole := &ovgs.RemoveUserRoleRequest{Username: username, UserType: ovgs.AccountType_ACCOUNT_TYPE_USER, OrgId: *orgID, GroupId: *groupID}
		t.Logf("Remove user Role req : %v", prettyPrint(deluserRole))
		_, _ = ovsC.RemoveUserRole(ctx, deluserRole)

		md := metadata.Pairs("Authorization", kgadwalToken)
		ctx := metadata.NewOutgoingContext(context.Background(), md)
		cc, err := DialGRPC(addr, ctx)
		if err != nil {
			log.Fatalf("Error in creating grpc connection %v", err)
		}
		ovsC = ovgs.NewOwnershipVoucherServiceClient(cc)
		//Remove Serial Number if exists
		removeReq := &ovgs.RemoveSerialRequest{GroupId: *groupID, Component: &ovgs.Component{SerialNumber: serialNumber}}
		removeRes, err := ovsC.RemoveSerial(ctx, removeReq)
		t.Logf("Remove  serial number request: %v", prettyPrint(removeReq))
		t.Logf("remove serial number response %v", removeRes.String())
		if (err != nil) && (!strings.Contains(err.Error(), "PermissionDenied desc = Insufficient privileges")) {
			t.Errorf("Error while deleting Serial Number %v ", err)
		}
	})
}
