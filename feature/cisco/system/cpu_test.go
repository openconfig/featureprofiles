/*
 Copyright 2022 Google LLC

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

      https://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package basetest

import (
	"testing"

	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/telemetry"
)

// /system/cpus/cpu/state/index
func TestCPUIndex(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/index", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpu_index := dut.Telemetry().System().Cpu(telemetry.Cpu_Index_Enum_ALL).Index().Get(t)
			if cpu_index == telemetry.Cpu_Index_Enum_ALL {
				t.Logf("Got correct CPU Index value")
			} else {
				t.Errorf("Unexpected CPU Index value: got: %v want: %v", cpu_index, telemetry.Cpu_Index_Enum_ALL)
			}
		})
	})
}

// /system/cpus/cpu/state/total
func TestCPUTotalInstant(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/total/instant", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpu_value := dut.Telemetry().System().Cpu(telemetry.Cpu_Index_Enum_ALL).Total().Instant().Get(t)
			if cpu_value > uint8(0) {
				t.Logf("Got correct CPU Idle Instant value")
			} else {
				t.Errorf("Unexpected CPU Idle Instant value,got: %v want: greater than %v", cpu_value, uint8(0))
			}
		})
	})
}

func TestCPUTotalAvg(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/total/avg", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpu_value := dut.Telemetry().System().Cpu(telemetry.Cpu_Index_Enum_ALL).Total().Avg().Get(t)
			if cpu_value > uint8(0) {
				t.Logf("Got correct CPU Idle Avg value")
			} else {
				t.Errorf("Unexpected CPU Idle Avg value,got: %v want: greater than %v", cpu_value, uint8(0))
			}
		})
	})
}

func TestCPUTotalMin(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/total/min", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpu_value := dut.Telemetry().System().Cpu(telemetry.Cpu_Index_Enum_ALL).Total().Min().Get(t)
			if cpu_value == uint8(0) {
				t.Logf("Got correct CPU Idle Min value")
			} else {
				t.Errorf("Unexpected CPU Idle Min value,got: %v want: %v", cpu_value, uint8(0))
			}
		})
	})
}

func TestCPUTotalMax(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/total/max", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpu_value := dut.Telemetry().System().Cpu(telemetry.Cpu_Index_Enum_ALL).Total().Max().Get(t)
			if cpu_value > uint8(0) {
				t.Logf("Got correct CPU Idle Max value")
			} else {
				t.Errorf("Unexpected CPU Idle Max value,got: %v want: greater than %v", cpu_value, uint8(0))
			}
		})
	})
}

func TestCPUTotalInterval(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/total/interval", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpu_value := dut.Telemetry().System().Cpu(telemetry.Cpu_Index_Enum_ALL).Total().Interval().Get(t)
			if cpu_value > uint64(0) {
				t.Logf("Got correct CPU Idle Interval value")
			} else {
				t.Errorf("Unexpected CPU Idle Interval value,got: %v want: greater than %v", cpu_value, uint64(0))
			}
		})
	})
}

func TestCPUTotalMinTime(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/total/mintime", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpu_value := dut.Telemetry().System().Cpu(telemetry.Cpu_Index_Enum_ALL).Total().MinTime().Get(t)
			if cpu_value > uint64(0) {
				t.Logf("Got correct CPU Idle MinTime value")
			} else {
				t.Errorf("Unexpected CPU Idle MinTime value,got: %v want: greater than %v", cpu_value, uint64(0))
			}
		})
	})
}

func TestCPUTotalMaxTime(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/total/maxtime", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpu_value := dut.Telemetry().System().Cpu(telemetry.Cpu_Index_Enum_ALL).Total().MaxTime().Get(t)
			if cpu_value > uint64(0) {
				t.Logf("Got correct CPU Idle Maxtime value")
			} else {
				t.Errorf("Unexpected CPU Idle Maxtime value,got: %v want: greater than %v", cpu_value, uint64(0))
			}
		})
	})
}

// /system/cpus/cpu/state/user
func TestCPUUserInstant(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/user/instant", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpu_value := dut.Telemetry().System().Cpu(telemetry.Cpu_Index_Enum_ALL).User().Instant().Get(t)
			if cpu_value > uint8(0) {
				t.Logf("Got correct CPU User Instant value")
			} else {
				t.Errorf("Unexpected CPU User Instant value,got: %v want: greater than %v", cpu_value, uint8(0))
			}
		})
	})
}

func TestCPUUserAvg(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/user/avg", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpu_value := dut.Telemetry().System().Cpu(telemetry.Cpu_Index_Enum_ALL).User().Avg().Get(t)
			if cpu_value > uint8(0) {
				t.Logf("Got correct CPU User Avg value")
			} else {
				t.Errorf("Unexpected CPU User Avg value,got: %v want: greater than %v", cpu_value, uint8(0))
			}
		})
	})
}

func TestCPUUserMin(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/user/min", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpu_value := dut.Telemetry().System().Cpu(telemetry.Cpu_Index_Enum_ALL).User().Min().Get(t)
			if cpu_value == uint8(0) {
				t.Logf("Got correct CPU User Min value")
			} else {
				t.Errorf("Unexpected CPU User Min value,got: %v want: %v", cpu_value, uint8(0))
			}
		})
	})
}

func TestCPUUserMax(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/user/max", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpu_value := dut.Telemetry().System().Cpu(telemetry.Cpu_Index_Enum_ALL).User().Max().Get(t)
			if cpu_value > uint8(0) {
				t.Logf("Got correct CPU User Max value")
			} else {
				t.Errorf("Unexpected CPU User Max value,got: %v want: greater than %v", cpu_value, uint8(0))
			}
		})
	})
}

func TestCPUUserInterval(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/user/interval", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpu_value := dut.Telemetry().System().Cpu(telemetry.Cpu_Index_Enum_ALL).User().Interval().Get(t)
			if cpu_value > uint64(0) {
				t.Logf("Got correct CPU User Interval value")
			} else {
				t.Errorf("Unexpected CPU User Interval value,got: %v want: greater than %v", cpu_value, uint64(0))
			}
		})
	})
}

func TestCPUUserMinTime(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/user/mintime", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpu_value := dut.Telemetry().System().Cpu(telemetry.Cpu_Index_Enum_ALL).User().MinTime().Get(t)
			if cpu_value > uint64(0) {
				t.Logf("Got correct CPU User MinTime value")
			} else {
				t.Errorf("Unexpected CPU User MinTime value,got: %v want: greater than %v", cpu_value, uint64(0))
			}
		})
	})
}

func TestCPUUserMaxTime(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/user/maxtime", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpu_value := dut.Telemetry().System().Cpu(telemetry.Cpu_Index_Enum_ALL).User().MaxTime().Get(t)
			if cpu_value > uint64(0) {
				t.Logf("Got correct CPU User Maxtime value")
			} else {
				t.Errorf("Unexpected CPU User Maxtime value,got: %v want: greater than %v", cpu_value, uint64(0))
			}
		})
	})
}

// /system/cpus/cpu/state/kernel
func TestCPUKernelInstant(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/kernel/instant", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpu_value := dut.Telemetry().System().Cpu(telemetry.Cpu_Index_Enum_ALL).Kernel().Instant().Get(t)
			if cpu_value > uint8(0) {
				t.Logf("Got correct CPU Kernel Instant value")
			} else {
				t.Errorf("Unexpected CPU Kernel Instant value,got: %v want: greater than %v", cpu_value, uint8(0))
			}
		})
	})
}

func TestCPUKernelAvg(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/kernel/avg", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpu_value := dut.Telemetry().System().Cpu(telemetry.Cpu_Index_Enum_ALL).Kernel().Avg().Get(t)
			if cpu_value > uint8(0) {
				t.Logf("Got correct CPU Kernel Avg value")
			} else {
				t.Errorf("Unexpected CPU Kernel Avg value,got: %v want: greater than %v", cpu_value, uint8(0))
			}
		})
	})
}

func TestCPUKernelMin(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/kernel/min", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpu_value := dut.Telemetry().System().Cpu(telemetry.Cpu_Index_Enum_ALL).Kernel().Min().Get(t)
			if cpu_value == uint8(0) {
				t.Logf("Got correct CPU Kernel Min value")
			} else {
				t.Errorf("Unexpected CPU Kernel Min value,got: %v want: %v", cpu_value, uint8(0))
			}
		})
	})
}

func TestCPUKernelMax(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/kernel/max", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpu_value := dut.Telemetry().System().Cpu(telemetry.Cpu_Index_Enum_ALL).Kernel().Max().Get(t)
			if cpu_value > uint8(0) {
				t.Logf("Got correct CPU Kernel Max value")
			} else {
				t.Errorf("Unexpected CPU Kernel Max value,got: %v want: greater than %v", cpu_value, uint8(0))
			}
		})
	})
}

func TestCPUKernelInterval(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/kernel/interval", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpu_value := dut.Telemetry().System().Cpu(telemetry.Cpu_Index_Enum_ALL).Kernel().Interval().Get(t)
			if cpu_value > uint64(0) {
				t.Logf("Got correct CPU Kernel Interval value")
			} else {
				t.Errorf("Unexpected CPU Kernel Interval value,got: %v want: greater than %v", cpu_value, uint64(0))
			}
		})
	})
}

func TestCPUKernelMinTime(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/kernel/mintime", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpu_value := dut.Telemetry().System().Cpu(telemetry.Cpu_Index_Enum_ALL).Kernel().MinTime().Get(t)
			if cpu_value > uint64(0) {
				t.Logf("Got correct CPU Kernel MinTime value")
			} else {
				t.Errorf("Unexpected CPU Kernel MinTime value,got: %v want: greater than %v", cpu_value, uint64(0))
			}
		})
	})
}

func TestCPUKernelMaxTime(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/kernel/maxtime", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpu_value := dut.Telemetry().System().Cpu(telemetry.Cpu_Index_Enum_ALL).Kernel().MaxTime().Get(t)
			if cpu_value > uint64(0) {
				t.Logf("Got correct CPU Kernel Maxtime value")
			} else {
				t.Errorf("Unexpected CPU Kernel Maxtime value,got: %v want: greater than %v", cpu_value, uint64(0))
			}
		})
	})
}

// /system/cpus/cpu/state/nice
func TestCPUNiceInstant(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/nice/instant", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpu_value := dut.Telemetry().System().Cpu(telemetry.Cpu_Index_Enum_ALL).Nice().Instant().Get(t)
			if cpu_value >= uint8(0) {
				t.Logf("Got correct CPU Nice Instant value")
			} else {
				t.Errorf("Unexpected CPU Nice Instant value,got: %v want: greater than %v", cpu_value, uint8(0))
			}
		})
	})
}

func TestCPUNiceAvg(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/nice/avg", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpu_value := dut.Telemetry().System().Cpu(telemetry.Cpu_Index_Enum_ALL).Nice().Avg().Get(t)
			if cpu_value >= uint8(0) {
				t.Logf("Got correct CPU Nice Avg value")
			} else {
				t.Errorf("Unexpected CPU Nice Avg value,got: %v want: greater than %v", cpu_value, uint8(0))
			}
		})
	})
}

func TestCPUNiceMin(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/nice/min", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpu_value := dut.Telemetry().System().Cpu(telemetry.Cpu_Index_Enum_ALL).Nice().Min().Get(t)
			if cpu_value == uint8(0) {
				t.Logf("Got correct CPU Nice Min value")
			} else {
				t.Errorf("Unexpected CPU Nice Min value,got: %v want: %v", cpu_value, uint8(0))
			}
		})
	})
}

func TestCPUNiceMax(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/nice/max", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpu_value := dut.Telemetry().System().Cpu(telemetry.Cpu_Index_Enum_ALL).Nice().Max().Get(t)
			if cpu_value >= uint8(0) {
				t.Logf("Got correct CPU Nice Max value")
			} else {
				t.Errorf("Unexpected CPU Nice Max value,got: %v want: greater than %v", cpu_value, uint8(0))
			}
		})
	})
}

func TestCPUNiceInterval(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/nice/interval", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpu_value := dut.Telemetry().System().Cpu(telemetry.Cpu_Index_Enum_ALL).Nice().Interval().Get(t)
			if cpu_value > uint64(0) {
				t.Logf("Got correct CPU Nice Interval value")
			} else {
				t.Errorf("Unexpected CPU Nice Interval value,got: %v want: greater than %v", cpu_value, uint64(0))
			}
		})
	})
}

func TestCPUNiceMinTime(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/nice/mintime", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpu_value := dut.Telemetry().System().Cpu(telemetry.Cpu_Index_Enum_ALL).Nice().MinTime().Get(t)
			if cpu_value > uint64(0) {
				t.Logf("Got correct CPU Nice MinTime value")
			} else {
				t.Errorf("Unexpected CPU Nice MinTime value,got: %v want: greater than %v", cpu_value, uint64(0))
			}
		})
	})
}

func TestCPUNiceMaxTime(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/nice/maxtime", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpu_value := dut.Telemetry().System().Cpu(telemetry.Cpu_Index_Enum_ALL).Nice().MaxTime().Get(t)
			if cpu_value > uint64(0) {
				t.Logf("Got correct CPU Nice Maxtime value")
			} else {
				t.Errorf("Unexpected CPU Nice Maxtime value,got: %v want: greater than %v", cpu_value, uint64(0))
			}
		})
	})
}

// /system/cpus/cpu/state/idle
func TestCPUIdleInstant(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/idle/instant", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpu_idle := dut.Telemetry().System().Cpu(telemetry.Cpu_Index_Enum_ALL).Idle().Instant().Get(t)
			if cpu_idle > uint8(0) {
				t.Logf("Got correct CPU Idle Instant value")
			} else {
				t.Errorf("Unexpected CPU Idle Instant value,got: %v want: greater than %v", cpu_idle, uint8(0))
			}
		})
	})
}

func TestCPUIdleAvg(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/idle/avg", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpu_idle := dut.Telemetry().System().Cpu(telemetry.Cpu_Index_Enum_ALL).Idle().Avg().Get(t)
			if cpu_idle > uint8(0) {
				t.Logf("Got correct CPU Idle Avg value")
			} else {
				t.Errorf("Unexpected CPU Idle Avg value,got: %v want: greater than %v", cpu_idle, uint8(0))
			}
		})
	})
}

func TestCPUIdleMin(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/idle/min", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpu_idle := dut.Telemetry().System().Cpu(telemetry.Cpu_Index_Enum_ALL).Idle().Min().Get(t)
			if cpu_idle == uint8(0) {
				t.Logf("Got correct CPU Idle Min value")
			} else {
				t.Errorf("Unexpected CPU Idle Min value,got: %v want: %v", cpu_idle, uint8(0))
			}
		})
	})
}

func TestCPUIdleMax(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/idle/max", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpu_idle := dut.Telemetry().System().Cpu(telemetry.Cpu_Index_Enum_ALL).Idle().Max().Get(t)
			if cpu_idle > uint8(0) {
				t.Logf("Got correct CPU Idle Max value")
			} else {
				t.Errorf("Unexpected CPU Idle Max value,got: %v want: greater than %v", cpu_idle, uint8(0))
			}
		})
	})
}

func TestCPUIdleInterval(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/idle/interval", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpu_idle := dut.Telemetry().System().Cpu(telemetry.Cpu_Index_Enum_ALL).Idle().Interval().Get(t)
			if cpu_idle > uint64(0) {
				t.Logf("Got correct CPU Idle Interval value")
			} else {
				t.Errorf("Unexpected CPU Idle Interval value,got: %v want: greater than %v", cpu_idle, uint64(0))
			}
		})
	})
}

func TestCPUIdleMinTime(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/idle/mintime", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpu_idle := dut.Telemetry().System().Cpu(telemetry.Cpu_Index_Enum_ALL).Idle().MinTime().Get(t)
			if cpu_idle > uint64(0) {
				t.Logf("Got correct CPU Idle MinTime value")
			} else {
				t.Errorf("Unexpected CPU Idle MinTime value,got: %v want: greater than %v", cpu_idle, uint64(0))
			}
		})
	})
}

func TestCPUIdleMaxTime(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/idle/maxtime", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpu_idle := dut.Telemetry().System().Cpu(telemetry.Cpu_Index_Enum_ALL).Idle().MaxTime().Get(t)
			if cpu_idle > uint64(0) {
				t.Logf("Got correct CPU Idle Maxtime value")
			} else {
				t.Errorf("Unexpected CPU Idle Maxtime value,got: %v want: greater than %v", cpu_idle, uint64(0))
			}
		})
	})
}

// /system/cpus/cpu/state/wait
func TestCPUWaitInstant(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/wait/instant", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpu_idle := dut.Telemetry().System().Cpu(telemetry.Cpu_Index_Enum_ALL).Wait().Instant().Get(t)
			if cpu_idle >= uint8(0) {
				t.Logf("Got correct CPU Wait Instant value")
			} else {
				t.Errorf("Unexpected CPU Wait Instant value,got: %v want: greater than %v", cpu_idle, uint8(0))
			}
		})
	})
}

func TestCPUWaitAvg(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/wait/avg", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpu_idle := dut.Telemetry().System().Cpu(telemetry.Cpu_Index_Enum_ALL).Wait().Avg().Get(t)
			if cpu_idle >= uint8(0) {
				t.Logf("Got correct CPU Wait Avg value")
			} else {
				t.Errorf("Unexpected CPU Wait Avg value,got: %v want: greater than %v", cpu_idle, uint8(0))
			}
		})
	})
}

func TestCPUWaitMin(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/wait/min", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpu_idle := dut.Telemetry().System().Cpu(telemetry.Cpu_Index_Enum_ALL).Wait().Min().Get(t)
			if cpu_idle == uint8(0) {
				t.Logf("Got correct CPU Wait Min value")
			} else {
				t.Errorf("Unexpected CPU Wait Min value,got: %v want: %v", cpu_idle, uint8(0))
			}
		})
	})
}

func TestCPUWaitMax(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/wait/max", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpu_idle := dut.Telemetry().System().Cpu(telemetry.Cpu_Index_Enum_ALL).Wait().Max().Get(t)
			if cpu_idle >= uint8(0) {
				t.Logf("Got correct CPU Wait Max value")
			} else {
				t.Errorf("Unexpected CPU Wait Max value,got: %v want: greater than %v", cpu_idle, uint8(0))
			}
		})
	})
}

func TestCPUWaitInterval(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/wait/interval", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpu_idle := dut.Telemetry().System().Cpu(telemetry.Cpu_Index_Enum_ALL).Wait().Interval().Get(t)
			if cpu_idle > uint64(0) {
				t.Logf("Got correct CPU Wait Interval value")
			} else {
				t.Errorf("Unexpected CPU Wait Interval value,got: %v want: greater than %v", cpu_idle, uint64(0))
			}
		})
	})
}

func TestCPUWaitMinTime(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/wait/mintime", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpu_idle := dut.Telemetry().System().Cpu(telemetry.Cpu_Index_Enum_ALL).Wait().MinTime().Get(t)
			if cpu_idle > uint64(0) {
				t.Logf("Got correct CPU Wait MinTime value")
			} else {
				t.Errorf("Unexpected CPU Wait MinTime value,got: %v want: greater than %v", cpu_idle, uint64(0))
			}
		})
	})
}

func TestCPUWaitMaxTime(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/wait/maxtime", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpu_idle := dut.Telemetry().System().Cpu(telemetry.Cpu_Index_Enum_ALL).Wait().MaxTime().Get(t)
			if cpu_idle > uint64(0) {
				t.Logf("Got correct CPU Wait Maxtime value")
			} else {
				t.Errorf("Unexpected CPU Wait Maxtime value,got: %v want: greater than %v", cpu_idle, uint64(0))
			}
		})
	})
}

// /system/cpus/cpu/state/hardware-interrupt
func TestCPUHardwareInterruptInstant(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/hardware-interrupt/instant", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpu_idle := dut.Telemetry().System().Cpu(telemetry.Cpu_Index_Enum_ALL).HardwareInterrupt().Instant().Get(t)
			if cpu_idle >= uint8(0) {
				t.Logf("Got correct CPU HardwareInterrupt Instant value")
			} else {
				t.Errorf("Unexpected CPU HardwareInterrupt Instant value,got: %v want: greater than %v", cpu_idle, uint8(0))
			}
		})
	})
}

func TestCPUHardwareInterruptAvg(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/hardware-interrupt/avg", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpu_idle := dut.Telemetry().System().Cpu(telemetry.Cpu_Index_Enum_ALL).HardwareInterrupt().Avg().Get(t)
			if cpu_idle >= uint8(0) {
				t.Logf("Got correct CPU HardwareInterrupt Avg value")
			} else {
				t.Errorf("Unexpected CPU HardwareInterrupt Avg value,got: %v want: greater than %v", cpu_idle, uint8(0))
			}
		})
	})
}

func TestCPUHardwareInterruptMin(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/hardware-interrupt/min", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpu_idle := dut.Telemetry().System().Cpu(telemetry.Cpu_Index_Enum_ALL).HardwareInterrupt().Min().Get(t)
			if cpu_idle == uint8(0) {
				t.Logf("Got correct CPU HardwareInterrupt Min value")
			} else {
				t.Errorf("Unexpected CPU HardwareInterrupt Min value,got: %v want: %v", cpu_idle, uint8(0))
			}
		})
	})
}

func TestCPUHardwareInterruptMax(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/hardware-interrupt/max", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpu_idle := dut.Telemetry().System().Cpu(telemetry.Cpu_Index_Enum_ALL).HardwareInterrupt().Max().Get(t)
			if cpu_idle >= uint8(0) {
				t.Logf("Got correct CPU HardwareInterrupt Max value")
			} else {
				t.Errorf("Unexpected CPU HardwareInterrupt Max value,got: %v want: greater than %v", cpu_idle, uint8(0))
			}
		})
	})
}

func TestCPUHardwareInterruptInterval(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/hardware-interrupt/interval", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpu_idle := dut.Telemetry().System().Cpu(telemetry.Cpu_Index_Enum_ALL).HardwareInterrupt().Interval().Get(t)
			if cpu_idle > uint64(0) {
				t.Logf("Got correct CPU WHardwareInterruptait Interval value")
			} else {
				t.Errorf("Unexpected CPU HardwareInterrupt Interval value,got: %v want: greater than %v", cpu_idle, uint64(0))
			}
		})
	})
}

func TestCPUHardwareInterruptMinTime(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/hardware-interrupt/mintime", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpu_idle := dut.Telemetry().System().Cpu(telemetry.Cpu_Index_Enum_ALL).HardwareInterrupt().MinTime().Get(t)
			if cpu_idle > uint64(0) {
				t.Logf("Got correct CPU HardwareInterrupt MinTime value")
			} else {
				t.Errorf("Unexpected CPU HardwareInterrupt MinTime value,got: %v want: greater than %v", cpu_idle, uint64(0))
			}
		})
	})
}

func TestCPUHardwareInterruptMaxTime(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/hardware-interrupt/maxtime", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpu_idle := dut.Telemetry().System().Cpu(telemetry.Cpu_Index_Enum_ALL).HardwareInterrupt().MaxTime().Get(t)
			if cpu_idle > uint64(0) {
				t.Logf("Got correct CPU HardwareInterrupt Maxtime value")
			} else {
				t.Errorf("Unexpected CPU HardwareInterrupt Maxtime value,got: %v want: greater than %v", cpu_idle, uint64(0))
			}
		})
	})
}

// /system/cpus/cpu/state/software-interrupt
func TestCPUSoftwareInterruptInstant(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/software-interrupt/instant", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpu_idle := dut.Telemetry().System().Cpu(telemetry.Cpu_Index_Enum_ALL).SoftwareInterrupt().Instant().Get(t)
			if cpu_idle >= uint8(0) {
				t.Logf("Got correct CPU SoftwareInterrupt Instant value")
			} else {
				t.Errorf("Unexpected CPU SoftwareInterrupt Instant value,got: %v want: greater than %v", cpu_idle, uint8(0))
			}
		})
	})
}

func TestCPUSoftwareInterruptAvg(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/software-interrupt/avg", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpu_idle := dut.Telemetry().System().Cpu(telemetry.Cpu_Index_Enum_ALL).SoftwareInterrupt().Avg().Get(t)
			if cpu_idle >= uint8(0) {
				t.Logf("Got correct CPU SoftwareInterrupt Avg value")
			} else {
				t.Errorf("Unexpected CPU SoftwareInterrupt Avg value,got: %v want: greater than %v", cpu_idle, uint8(0))
			}
		})
	})
}

func TestCPUSoftwareInterruptMin(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/software-interrupt/min", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpu_idle := dut.Telemetry().System().Cpu(telemetry.Cpu_Index_Enum_ALL).SoftwareInterrupt().Min().Get(t)
			if cpu_idle == uint8(0) {
				t.Logf("Got correct CPU SoftwareInterrupt Min value")
			} else {
				t.Errorf("Unexpected CPU SoftwareInterrupt Min value,got: %v want: %v", cpu_idle, uint8(0))
			}
		})
	})
}

func TestCPUSoftwareInterruptMax(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/software-interrupt/max", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpu_idle := dut.Telemetry().System().Cpu(telemetry.Cpu_Index_Enum_ALL).SoftwareInterrupt().Max().Get(t)
			if cpu_idle >= uint8(0) {
				t.Logf("Got correct CPU SoftwareInterrupt Max value")
			} else {
				t.Errorf("Unexpected CPU SoftwareInterrupt Max value,got: %v want: greater than %v", cpu_idle, uint8(0))
			}
		})
	})
}

func TestCPUSoftwareInterruptInterval(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/software-interrupt/interval", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpu_idle := dut.Telemetry().System().Cpu(telemetry.Cpu_Index_Enum_ALL).SoftwareInterrupt().Interval().Get(t)
			if cpu_idle > uint64(0) {
				t.Logf("Got correct CPU SoftwareInterrupt Interval value")
			} else {
				t.Errorf("Unexpected CPU SoftwareInterrupt Interval value,got: %v want: greater than %v", cpu_idle, uint64(0))
			}
		})
	})
}

func TestCPUSoftwareInterruptMinTime(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/software-interrupt/mintime", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpu_idle := dut.Telemetry().System().Cpu(telemetry.Cpu_Index_Enum_ALL).SoftwareInterrupt().MinTime().Get(t)
			if cpu_idle > uint64(0) {
				t.Logf("Got correct CPU SoftwareInterrupt MinTime value")
			} else {
				t.Errorf("Unexpected CPU SoftwareInterrupt MinTime value,got: %v want: greater than %v", cpu_idle, uint64(0))
			}
		})
	})
}

func TestCPUSoftwareInterruptMaxTime(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/software-interrupt/maxtime", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpu_idle := dut.Telemetry().System().Cpu(telemetry.Cpu_Index_Enum_ALL).SoftwareInterrupt().MaxTime().Get(t)
			if cpu_idle > uint64(0) {
				t.Logf("Got correct CPU SoftwareInterrupt Maxtime value")
			} else {
				t.Errorf("Unexpected CPU SoftwareInterrupt Maxtime value,got: %v want: greater than %v", cpu_idle, uint64(0))
			}
		})
	})
}
