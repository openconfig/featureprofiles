package basetest

import (
	"testing"

	"github.com/openconfig/featureprofiles/internal/fptest"
)

type Testcase struct {
	name string
	desc string
	fn   func(t *testing.T)
}

var (
	TimeTestcases = []Testcase{
		{
			name: "testBootTime",
			desc: "testBootTime",
			fn:   testBootTime,
		},
	}
	SystemTestcases = []Testcase{
		{
			name: "testSystemContainerUpdate",
			desc: "testSystemContainerUpdate",
			fn:   testSystemContainerUpdate,
		},
		{
			name: "testSysGrpcState",
			desc: "testSysGrpcState",
			fn:   testSysGrpcState,
		},
		{
			name: "testSysGrpcConfig",
			desc: "testSysGrpcConfig",
			fn:   testSysGrpcConfig,
		},
		{
			name: "testGrpcListenAddress",
			desc: "testGrpcListenAddress",
			fn:   testGrpcListenAddress,
		},
	}
	CpuTestcases = []Testcase{
		{
			name: "testCPUIndex",
			desc: "testCPUIndex",
			fn:   testCPUIndex,
		},
		{
			name: "testCPUTotalInstant",
			desc: "testCPUTotalInstant",
			fn:   testCPUTotalInstant,
		},
		{
			name: "testCPUTotalAvg",
			desc: "testCPUTotalAvg",
			fn:   testCPUTotalAvg,
		},
		{
			name: "testCPUTotalMin",
			desc: "testCPUTotalMin",
			fn:   testCPUTotalMin,
		},
		{
			name: "testCPUTotalMax",
			desc: "testCPUTotalMax",
			fn:   testCPUTotalMax,
		},
		{
			name: "testCPUTotalInterval",
			desc: "testCPUTotalInterval",
			fn:   testCPUTotalInterval,
		},
		{
			name: "testCPUTotalMinTime",
			desc: "testCPUTotalMinTime",
			fn:   testCPUTotalMinTime,
		},
		{
			name: "testCPUTotalMaxTime",
			desc: "testCPUTotalMaxTime",
			fn:   testCPUTotalMaxTime,
		},
		{
			name: "testCPUUserInstant",
			desc: "testCPUUserInstant",
			fn:   testCPUUserInstant,
		},
		{
			name: "testCPUUserAvg",
			desc: "testCPUUserAvg",
			fn:   testCPUUserAvg,
		},
		{
			name: "testCPUUserMin",
			desc: "testCPUUserMin",
			fn:   testCPUUserMin,
		},
		{
			name: "testCPUUserMax",
			desc: "testCPUUserMax",
			fn:   testCPUUserMax,
		},
		{
			name: "testCPUUserInterval",
			desc: "testCPUUserInterval",
			fn:   testCPUUserInterval,
		},
		{
			name: "testCPUUserMinTime",
			desc: "testCPUUserMinTime",
			fn:   testCPUUserMinTime,
		},
		{
			name: "testCPUUserMaxTime",
			desc: "testCPUUserMaxTime",
			fn:   testCPUUserMaxTime,
		},
		{
			name: "testCPUKernelInstant",
			desc: "testCPUKernelInstant",
			fn:   testCPUKernelInstant,
		},
		{
			name: "testCPUKernelAvg",
			desc: "testCPUKernelAvg",
			fn:   testCPUKernelAvg,
		},
		{
			name: "testCPUKernelMin",
			desc: "testCPUKernelMin",
			fn:   testCPUKernelMin,
		},
		{
			name: "testCPUKernelMax",
			desc: "testCPUKernelMax",
			fn:   testCPUKernelMax,
		},
		{
			name: "testCPUKernelInterval",
			desc: "testCPUKernelInterval",
			fn:   testCPUKernelInterval,
		},
		{
			name: "testCPUKernelMinTime",
			desc: "testCPUKernelMinTime",
			fn:   testCPUKernelMinTime,
		},
		{
			name: "testCPUKernelMaxTime",
			desc: "testCPUKernelMaxTime",
			fn:   testCPUKernelMaxTime,
		},
		{
			name: "testCPUNiceInstant",
			desc: "testCPUNiceInstant",
			fn:   testCPUNiceInstant,
		},
		{
			name: "testCPUNiceAvg",
			desc: "testCPUNiceAvg",
			fn:   testCPUNiceAvg,
		},
		{
			name: "testCPUNiceMin",
			desc: "testCPUNiceMin",
			fn:   testCPUNiceMin,
		},
		{
			name: "testCPUNiceMax",
			desc: "testCPUNiceMax",
			fn:   testCPUNiceMax,
		},
		{
			name: "testCPUNiceInterval",
			desc: "testCPUNiceInterval",
			fn:   testCPUNiceInterval,
		},
		{
			name: "testCPUNiceMinTime",
			desc: "testCPUNiceMinTime",
			fn:   testCPUNiceMinTime,
		},
		{
			name: "testCPUNiceMaxTime",
			desc: "testCPUNiceMaxTime",
			fn:   testCPUNiceMaxTime,
		},
		{
			name: "testCPUIdleInstant",
			desc: "testCPUIdleInstant",
			fn:   testCPUIdleInstant,
		},
		{
			name: "testCPUIdleAvg",
			desc: "testCPUIdleAvg",
			fn:   testCPUIdleAvg,
		},
		{
			name: "testCPUIdleMin",
			desc: "testCPUIdleMin",
			fn:   testCPUIdleMin,
		},
		{
			name: "testCPUIdleMax",
			desc: "testCPUIdleMax",
			fn:   testCPUIdleMax,
		},
		{
			name: "testCPUIdleInterval",
			desc: "testCPUIdleInterval",
			fn:   testCPUIdleInterval,
		},
		{
			name: "testCPUIdleMinTime",
			desc: "testCPUIdleMinTime",
			fn:   testCPUIdleMinTime,
		},
		{
			name: "testCPUIdleMaxTime",
			desc: "testCPUIdleMaxTime",
			fn:   testCPUIdleMaxTime,
		},
		{
			name: "testCPUWaitInstant",
			desc: "testCPUWaitInstant",
			fn:   testCPUWaitInstant,
		},
		{
			name: "testCPUWaitAvg",
			desc: "testCPUWaitAvg",
			fn:   testCPUWaitAvg,
		},
		{
			name: "testCPUWaitMin",
			desc: "testCPUWaitMin",
			fn:   testCPUWaitMin,
		},
		{
			name: "testCPUWaitMax",
			desc: "testCPUWaitMax",
			fn:   testCPUWaitMax,
		},
		{
			name: "testCPUWaitInterval",
			desc: "testCPUWaitInterval",
			fn:   testCPUWaitInterval,
		},
		{
			name: "testCPUWaitMinTime",
			desc: "testCPUWaitMinTime",
			fn:   testCPUWaitMinTime,
		},
		{
			name: "testCPUWaitMaxTime",
			desc: "testCPUWaitMaxTime",
			fn:   testCPUWaitMaxTime,
		},
		{
			name: "testCPUHardwareInterruptInstant",
			desc: "testCPUHardwareInterruptInstant",
			fn:   testCPUHardwareInterruptInstant,
		},
		{
			name: "testCPUHardwareInterruptAvg",
			desc: "testCPUHardwareInterruptAvg",
			fn:   testCPUHardwareInterruptAvg,
		},
		{
			name: "testCPUHardwareInterruptMin",
			desc: "testCPUHardwareInterruptMin",
			fn:   testCPUHardwareInterruptMin,
		},
		{
			name: "testCPUHardwareInterruptMax",
			desc: "testCPUHardwareInterruptMax",
			fn:   testCPUHardwareInterruptMax,
		},
		{
			name: "testCPUHardwareInterruptInterval",
			desc: "testCPUHardwareInterruptInterval",
			fn:   testCPUHardwareInterruptInterval,
		},
		{
			name: "testCPUHardwareInterruptMinTime",
			desc: "testCPUHardwareInterruptMinTime",
			fn:   testCPUHardwareInterruptMinTime,
		},
		{
			name: "testCPUHardwareInterruptMaxTime",
			desc: "testCPUHardwareInterruptMaxTime",
			fn:   testCPUHardwareInterruptMaxTime,
		},
		{
			name: "testCPUSoftwareInterruptInstant",
			desc: "testCPUSoftwareInterruptInstant",
			fn:   testCPUSoftwareInterruptInstant,
		},
		{
			name: "testCPUSoftwareInterruptAvg",
			desc: "testCPUSoftwareInterruptAvg",
			fn:   testCPUSoftwareInterruptAvg,
		},
		{
			name: "testCPUSoftwareInterruptMin",
			desc: "testCPUSoftwareInterruptMin",
			fn:   testCPUSoftwareInterruptMin,
		},
		{
			name: "testCPUSoftwareInterruptMax",
			desc: "testCPUSoftwareInterruptMax",
			fn:   testCPUSoftwareInterruptMax,
		},
		{
			name: "testCPUSoftwareInterruptInterval",
			desc: "testCPUSoftwareInterruptInterval",
			fn:   testCPUSoftwareInterruptInterval,
		},
		{
			name: "testCPUSoftwareInterruptMinTime",
			desc: "testCPUSoftwareInterruptMinTime",
			fn:   testCPUSoftwareInterruptMinTime,
		},
		{
			name: "testCPUSoftwareInterruptMaxTime",
			desc: "testCPUSoftwareInterruptMaxTime",
			fn:   testCPUSoftwareInterruptMaxTime,
		},
	}
	HostNameTestcases = []Testcase{
		{
			name: "testHostname",
			desc: "testHostname",
			fn:   testHostname,
		},
	}
	IanaportsTestcases = []Testcase{
		{
			name: "test Iana Ports",
			desc: "test Iana Ports",
			fn:   testIanaPorts,
		},
	}
	MemoryTestcases = []Testcase{
		{
			name: "testMemoryReserved",
			desc: "testMemoryReserved",
			fn:   testMemoryReserved,
		},
		{
			name: "testMemoryPhysical",
			desc: "testMemoryPhysical",
			fn:   testMemoryPhysical,
		},
	}
	NtpTestcases = []Testcase{
		{
			name: "testNTPEnableConfig",
			desc: "testNTPEnableConfig",
			fn:   testNTPEnableConfig,
		},
		{
			name: "testNTPEnableState",
			desc: "testNTPEnableState",
			fn:   testNTPEnableState,
		},
	}
	SshTestcases = []Testcase{
		{
			name: "testSSHServerEnableConfig",
			desc: "testSSHServerEnableConfig",
			fn:   testSSHServerEnableConfig,
		},
		{
			name: "testSSHEnableState",
			desc: "testSSHEnableState",
			fn:   testSSHEnableState,
		},
	}
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestSys(t *testing.T) {
	TestCasesList := [][]Testcase{
		TimeTestcases,
		SystemTestcases, // SystemTestcase should be always before IanaportsTestcases
		CpuTestcases,
		HostNameTestcases,
		IanaportsTestcases,
		MemoryTestcases,
		NtpTestcases,
		SshTestcases,
	}
	for _, TestCases := range TestCasesList {
		for _, tt := range TestCases {
			runner(t, tt)
		}
	}
}

func runner(t *testing.T, tt Testcase) {
	t.Run(tt.name, func(t *testing.T) {
		t.Logf("Name: %s", tt.name)
		t.Logf("Description: %s", tt.desc)
		tt.fn(t)
	})
}
