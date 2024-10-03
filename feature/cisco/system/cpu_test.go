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
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
)

// /system/cpus/cpu/state/index
func testCPUIndex(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/index", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpuIndex := gnmi.Get(t, dut, gnmi.OC().System().Cpu(oc.Cpu_Index_ALL).Index().State())
			if cpuIndex == oc.Cpu_Index_ALL {
				t.Logf("Got correct CPU Index value")
			} else {
				t.Errorf("Unexpected CPU Index value: got: %v want: %v", cpuIndex, oc.Cpu_Index_ALL)
			}
		})
	})
}

// /system/cpus/cpu/state/total
func testCPUTotalInstant(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/total/instant", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpuValue := gnmi.Get(t, dut, gnmi.OC().System().Cpu(oc.Cpu_Index_ALL).Total().Instant().State())
			if cpuValue == uint8(0) || cpuValue > uint8(0) {
				t.Logf("Got correct CPU Idle Instant value")
			} else {
				t.Errorf("Unexpected CPU Idle Instant value,got: %v want: greater than or equal to %v", cpuValue, uint8(0))
			}
		})
	})
}

func testCPUTotalAvg(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/total/avg", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpuValue := gnmi.Get(t, dut, gnmi.OC().System().Cpu(oc.Cpu_Index_ALL).Total().Avg().State())
			if cpuValue == uint8(0) || cpuValue > uint8(0) {
				t.Logf("Got correct CPU Idle Avg value")
			} else {
				t.Errorf("Unexpected CPU Idle Avg value,got: %v want: greater than or equal to %v", cpuValue, uint8(0))
			}
		})
	})
}

func testCPUTotalMin(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/total/min", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpuValue := gnmi.Get(t, dut, gnmi.OC().System().Cpu(oc.Cpu_Index_ALL).Total().Min().State())
			if cpuValue == uint8(0) || cpuValue > uint8(0) {
				t.Logf("Got correct CPU Idle Min value")
			} else {
				t.Errorf("Unexpected CPU Idle Min value,got: %v want: greater than or equal to %v", cpuValue, uint8(0))
			}
		})
	})
}

func testCPUTotalMax(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/total/max", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpuValue := gnmi.Get(t, dut, gnmi.OC().System().Cpu(oc.Cpu_Index_ALL).Total().Max().State())
			if cpuValue == uint8(0) || cpuValue > uint8(0) {
				t.Logf("Got correct CPU Idle Max value")
			} else {
				t.Errorf("Unexpected CPU Idle Max value,got: %v want: greater than or equal to %v", cpuValue, uint8(0))
			}
		})
	})
}

func testCPUTotalInterval(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/total/interval", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpuValue := gnmi.Get(t, dut, gnmi.OC().System().Cpu(oc.Cpu_Index_ALL).Total().Interval().State())
			if cpuValue == uint64(0) || cpuValue > uint64(0) {
				t.Logf("Got correct CPU Idle Interval value")
			} else {
				t.Errorf("Unexpected CPU Idle Interval value,got: %v want: greater than or equal to %v", cpuValue, uint64(0))
			}
		})
	})
}

func testCPUTotalMinTime(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/total/mintime", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpuValue := gnmi.Get(t, dut, gnmi.OC().System().Cpu(oc.Cpu_Index_ALL).Total().MinTime().State())
			if cpuValue == uint64(0) || cpuValue > uint64(0) {
				t.Logf("Got correct CPU Idle MinTime value")
			} else {
				t.Errorf("Unexpected CPU Idle MinTime value,got: %v want: greater than or equal to %v", cpuValue, uint64(0))
			}
		})
	})
}

func testCPUTotalMaxTime(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/total/maxtime", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpuValue := gnmi.Get(t, dut, gnmi.OC().System().Cpu(oc.Cpu_Index_ALL).Total().MaxTime().State())
			if cpuValue == uint64(0) || cpuValue > uint64(0) {
				t.Logf("Got correct CPU Idle Maxtime value")
			} else {
				t.Errorf("Unexpected CPU Idle Maxtime value,got: %v want: greater than  or equal to %v", cpuValue, uint64(0))
			}
		})
	})
}

// /system/cpus/cpu/state/user
func testCPUUserInstant(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/user/instant", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpuValue := gnmi.Get(t, dut, gnmi.OC().System().Cpu(oc.Cpu_Index_ALL).User().Instant().State())
			if cpuValue == uint8(0) || cpuValue > uint8(0) {
				t.Logf("Got correct CPU User Instant value")
			} else {
				t.Errorf("Unexpected CPU User Instant value,got: %v want: greater than or equal to %v", cpuValue, uint8(0))
			}
		})
	})
}

func testCPUUserAvg(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/user/avg", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpuValue := gnmi.Get(t, dut, gnmi.OC().System().Cpu(oc.Cpu_Index_ALL).User().Avg().State())
			if cpuValue == uint8(0) || cpuValue > uint8(0) {
				t.Logf("Got correct CPU User Avg value")
			} else {
				t.Errorf("Unexpected CPU User Avg value,got: %v want: greater than or equal to %v", cpuValue, uint8(0))
			}
		})
	})
}

func testCPUUserMin(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/user/min", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpuValue := gnmi.Get(t, dut, gnmi.OC().System().Cpu(oc.Cpu_Index_ALL).User().Min().State())
			if cpuValue == uint8(0) || cpuValue > uint8(0) {
				t.Logf("Got correct CPU User Min value")
			} else {
				t.Errorf("Unexpected CPU User Min value,got: %v want: greater than or equal to %v", cpuValue, uint8(0))
			}
		})
	})
}

func testCPUUserMax(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/user/max", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpuValue := gnmi.Get(t, dut, gnmi.OC().System().Cpu(oc.Cpu_Index_ALL).User().Max().State())
			if cpuValue == uint8(0) || cpuValue > uint8(0) {
				t.Logf("Got correct CPU User Max value")
			} else {
				t.Errorf("Unexpected CPU User Max value,got: %v want: greater than or equal to %v", cpuValue, uint8(0))
			}
		})
	})
}

func testCPUUserInterval(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/user/interval", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpuValue := gnmi.Get(t, dut, gnmi.OC().System().Cpu(oc.Cpu_Index_ALL).User().Interval().State())
			if cpuValue == uint64(0) || cpuValue > uint64(0) {
				t.Logf("Got correct CPU User Interval value")
			} else {
				t.Errorf("Unexpected CPU User Interval value,got: %v want: greater than or equal to %v", cpuValue, uint64(0))
			}
		})
	})
}

func testCPUUserMinTime(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/user/mintime", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpuValue := gnmi.Get(t, dut, gnmi.OC().System().Cpu(oc.Cpu_Index_ALL).User().MinTime().State())
			if cpuValue == uint64(0) || cpuValue > uint64(0) {
				t.Logf("Got correct CPU User MinTime value")
			} else {
				t.Errorf("Unexpected CPU User MinTime value,got: %v want: greater than or equal to %v", cpuValue, uint64(0))
			}
		})
	})
}

func testCPUUserMaxTime(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/user/maxtime", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpuValue := gnmi.Get(t, dut, gnmi.OC().System().Cpu(oc.Cpu_Index_ALL).User().MaxTime().State())
			if cpuValue == uint64(0) || cpuValue > uint64(0) {
				t.Logf("Got correct CPU User Maxtime value")
			} else {
				t.Errorf("Unexpected CPU User Maxtime value,got: %v want: greater than  or equal to %v", cpuValue, uint64(0))
			}
		})
	})
}

// /system/cpus/cpu/state/kernel
func testCPUKernelInstant(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/kernel/instant", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpuValue := gnmi.Get(t, dut, gnmi.OC().System().Cpu(oc.Cpu_Index_ALL).Kernel().Instant().State())
			if cpuValue == uint8(0) || cpuValue > uint8(0) {
				t.Logf("Got correct CPU Kernel Instant value")
			} else {
				t.Errorf("Unexpected CPU Kernel Instant value,got: %v want: greater than  or equal to %v", cpuValue, uint8(0))
			}
		})
	})
}

func testCPUKernelAvg(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/kernel/avg", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpuValue := gnmi.Get(t, dut, gnmi.OC().System().Cpu(oc.Cpu_Index_ALL).Kernel().Avg().State())
			if cpuValue == uint8(0) || cpuValue > uint8(0) {
				t.Logf("Got correct CPU Kernel Avg value")
			} else {
				t.Errorf("Unexpected CPU Kernel Avg value,got: %v want: greater than  or equal to %v", cpuValue, uint8(0))
			}
		})
	})
}

func testCPUKernelMin(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/kernel/min", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpuValue := gnmi.Get(t, dut, gnmi.OC().System().Cpu(oc.Cpu_Index_ALL).Kernel().Min().State())
			if cpuValue == uint8(0) || cpuValue > uint8(0) {
				t.Logf("Got correct CPU Kernel Min value")
			} else {
				t.Errorf("Unexpected CPU Kernel Min value,got: %v want: greater than  or equal to %v", cpuValue, uint8(0))
			}
		})
	})
}

func testCPUKernelMax(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/kernel/max", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpuValue := gnmi.Get(t, dut, gnmi.OC().System().Cpu(oc.Cpu_Index_ALL).Kernel().Max().State())
			if cpuValue == uint8(0) || cpuValue > uint8(0) {
				t.Logf("Got correct CPU Kernel Max value")
			} else {
				t.Errorf("Unexpected CPU Kernel Max value,got: %v want: greater than  or equal to %v", cpuValue, uint8(0))
			}
		})
	})
}

func testCPUKernelInterval(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/kernel/interval", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpuValue := gnmi.Get(t, dut, gnmi.OC().System().Cpu(oc.Cpu_Index_ALL).Kernel().Interval().State())
			if cpuValue == uint64(0) || cpuValue > uint64(0) {
				t.Logf("Got correct CPU Kernel Interval value")
			} else {
				t.Errorf("Unexpected CPU Kernel Interval value,got: %v want: greater than  or equal to %v", cpuValue, uint64(0))
			}
		})
	})
}

func testCPUKernelMinTime(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/kernel/mintime", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpuValue := gnmi.Get(t, dut, gnmi.OC().System().Cpu(oc.Cpu_Index_ALL).Kernel().MinTime().State())
			if cpuValue == uint64(0) || cpuValue > uint64(0) {
				t.Logf("Got correct CPU Kernel MinTime value")
			} else {
				t.Errorf("Unexpected CPU Kernel MinTime value,got: %v want: greater than  or equal to %v", cpuValue, uint64(0))
			}
		})
	})
}

func testCPUKernelMaxTime(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/kernel/maxtime", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpuValue := gnmi.Get(t, dut, gnmi.OC().System().Cpu(oc.Cpu_Index_ALL).Kernel().MaxTime().State())
			if cpuValue == uint64(0) || cpuValue > uint64(0) {
				t.Logf("Got correct CPU Kernel Maxtime value")
			} else {
				t.Errorf("Unexpected CPU Kernel Maxtime value,got: %v want: greater than  or equal to %v", cpuValue, uint64(0))
			}
		})
	})
}

// /system/cpus/cpu/state/nice
func testCPUNiceInstant(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/nice/instant", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpuValue := gnmi.Get(t, dut, gnmi.OC().System().Cpu(oc.Cpu_Index_ALL).Nice().Instant().State())
			if cpuValue == uint8(0) || cpuValue > uint8(0) {
				t.Logf("Got correct CPU Nice Instant value")
			} else {
				t.Errorf("Unexpected CPU Nice Instant value,got: %v want: greater than  or equal to %v", cpuValue, uint8(0))
			}
		})
	})
}

func testCPUNiceAvg(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/nice/avg", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpuValue := gnmi.Get(t, dut, gnmi.OC().System().Cpu(oc.Cpu_Index_ALL).Nice().Avg().State())
			if cpuValue == uint8(0) || cpuValue > uint8(0) {
				t.Logf("Got correct CPU Nice Avg value")
			} else {
				t.Errorf("Unexpected CPU Nice Avg value,got: %v want: greater than  or equal to %v", cpuValue, uint8(0))
			}
		})
	})
}

func testCPUNiceMin(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/nice/min", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpuValue := gnmi.Get(t, dut, gnmi.OC().System().Cpu(oc.Cpu_Index_ALL).Nice().Min().State())
			if cpuValue == uint8(0) || cpuValue > uint8(0) {
				t.Logf("Got correct CPU Nice Min value")
			} else {
				t.Errorf("Unexpected CPU Nice Min value,got: %v want: greater than  or equal to %v", cpuValue, uint8(0))
			}
		})
	})
}

func testCPUNiceMax(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/nice/max", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpuValue := gnmi.Get(t, dut, gnmi.OC().System().Cpu(oc.Cpu_Index_ALL).Nice().Max().State())
			if cpuValue == uint8(0) || cpuValue > uint8(0) {
				t.Logf("Got correct CPU Nice Max value")
			} else {
				t.Errorf("Unexpected CPU Nice Max value,got: %v want: greater than or equal to %v", cpuValue, uint8(0))
			}
		})
	})
}

func testCPUNiceInterval(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/nice/interval", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpuValue := gnmi.Get(t, dut, gnmi.OC().System().Cpu(oc.Cpu_Index_ALL).Nice().Interval().State())
			if cpuValue == uint64(0) || cpuValue > uint64(0) {
				t.Logf("Got correct CPU Nice Interval value")
			} else {
				t.Errorf("Unexpected CPU Nice Interval value,got: %v want: greater than or equal to %v", cpuValue, uint64(0))
			}
		})
	})
}

func testCPUNiceMinTime(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/nice/mintime", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpuValue := gnmi.Get(t, dut, gnmi.OC().System().Cpu(oc.Cpu_Index_ALL).Nice().MinTime().State())
			if cpuValue == uint64(0) || cpuValue > uint64(0) {
				t.Logf("Got correct CPU Nice MinTime value")
			} else {
				t.Errorf("Unexpected CPU Nice MinTime value,got: %v want: greater than  or equal to %v", cpuValue, uint64(0))
			}
		})
	})
}

func testCPUNiceMaxTime(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/nice/maxtime", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpuValue := gnmi.Get(t, dut, gnmi.OC().System().Cpu(oc.Cpu_Index_ALL).Nice().MaxTime().State())
			if cpuValue == uint64(0) || cpuValue > uint64(0) {
				t.Logf("Got correct CPU Nice Maxtime value")
			} else {
				t.Errorf("Unexpected CPU Nice Maxtime value,got: %v want: greater than or equal to %v", cpuValue, uint64(0))
			}
		})
	})
}

// /system/cpus/cpu/state/idle
func testCPUIdleInstant(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/idle/instant", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpuIdle := gnmi.Get(t, dut, gnmi.OC().System().Cpu(oc.Cpu_Index_ALL).Idle().Instant().State())
			if cpuIdle == uint8(0) || cpuIdle > uint8(0) {
				t.Logf("Got correct CPU Idle Instant value")
			} else {
				t.Errorf("Unexpected CPU Idle Instant value,got: %v want: greater than or equal to %v", cpuIdle, uint8(0))
			}
		})
	})
}

func testCPUIdleAvg(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/idle/avg", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpuIdle := gnmi.Get(t, dut, gnmi.OC().System().Cpu(oc.Cpu_Index_ALL).Idle().Avg().State())
			if cpuIdle == uint8(0) || cpuIdle > uint8(0) {
				t.Logf("Got correct CPU Idle Avg value")
			} else {
				t.Errorf("Unexpected CPU Idle Avg value,got: %v want: greater than or equal to %v", cpuIdle, uint8(0))
			}
		})
	})
}

func testCPUIdleMin(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/idle/min", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpuIdle := gnmi.Get(t, dut, gnmi.OC().System().Cpu(oc.Cpu_Index_ALL).Idle().Min().State())
			if cpuIdle == uint8(0) || cpuIdle > uint8(0) {
				t.Logf("Got correct CPU Idle Min value")
			} else {
				t.Errorf("Unexpected CPU Idle Min value,got: %v want: greater than or equal to %v", cpuIdle, uint8(0))
			}
		})
	})
}

func testCPUIdleMax(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/idle/max", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpuIdle := gnmi.Get(t, dut, gnmi.OC().System().Cpu(oc.Cpu_Index_ALL).Idle().Max().State())
			if cpuIdle == uint8(0) || cpuIdle > uint8(0) {
				t.Logf("Got correct CPU Idle Max value")
			} else {
				t.Errorf("Unexpected CPU Idle Max value,got: %v want: greater than or equal to  %v", cpuIdle, uint8(0))
			}
		})
	})
}

func testCPUIdleInterval(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/idle/interval", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpuIdle := gnmi.Get(t, dut, gnmi.OC().System().Cpu(oc.Cpu_Index_ALL).Idle().Interval().State())
			if cpuIdle == uint64(0) || cpuIdle > uint64(0) {
				t.Logf("Got correct CPU Idle Interval value")
			} else {
				t.Errorf("Unexpected CPU Idle Interval value,got: %v want: greater than or equal to  %v", cpuIdle, uint64(0))
			}
		})
	})
}

func testCPUIdleMinTime(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/idle/mintime", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpuIdle := gnmi.Get(t, dut, gnmi.OC().System().Cpu(oc.Cpu_Index_ALL).Idle().MinTime().State())
			if cpuIdle == uint64(0) || cpuIdle > uint64(0) {
				t.Logf("Got correct CPU Idle MinTime value")
			} else {
				t.Errorf("Unexpected CPU Idle MinTime value,got: %v want: greater than or equal to %v", cpuIdle, uint64(0))
			}
		})
	})
}

func testCPUIdleMaxTime(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/idle/maxtime", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpuIdle := gnmi.Get(t, dut, gnmi.OC().System().Cpu(oc.Cpu_Index_ALL).Idle().MaxTime().State())
			if cpuIdle == uint64(0) || cpuIdle > uint64(0) {
				t.Logf("Got correct CPU Idle Maxtime value")
			} else {
				t.Errorf("Unexpected CPU Idle Maxtime value,got: %v want: greater than or equal to %v", cpuIdle, uint64(0))
			}
		})
	})
}

// /system/cpus/cpu/state/wait
func testCPUWaitInstant(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/wait/instant", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpuIdle := gnmi.Get(t, dut, gnmi.OC().System().Cpu(oc.Cpu_Index_ALL).Wait().Instant().State())
			if cpuIdle == uint8(0) || cpuIdle > uint8(0) {
				t.Logf("Got correct CPU Wait Instant value")
			} else {
				t.Errorf("Unexpected CPU Wait Instant value,got: %v want: greater than or equal to %v", cpuIdle, uint8(0))
			}
		})
	})
}

func testCPUWaitAvg(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/wait/avg", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpuIdle := gnmi.Get(t, dut, gnmi.OC().System().Cpu(oc.Cpu_Index_ALL).Wait().Avg().State())
			if cpuIdle == uint8(0) || cpuIdle > uint8(0) {
				t.Logf("Got correct CPU Wait Avg value")
			} else {
				t.Errorf("Unexpected CPU Wait Avg value,got: %v want: greater than or equal to  %v", cpuIdle, uint8(0))
			}
		})
	})
}

func testCPUWaitMin(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/wait/min", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpuIdle := gnmi.Get(t, dut, gnmi.OC().System().Cpu(oc.Cpu_Index_ALL).Wait().Min().State())
			if cpuIdle == uint8(0) || cpuIdle > uint8(0) {
				t.Logf("Got correct CPU Wait Min value")
			} else {
				t.Errorf("Unexpected CPU Wait Min value,got: %v want: greater than or equal to  %v", cpuIdle, uint8(0))
			}
		})
	})
}

func testCPUWaitMax(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/wait/max", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpuIdle := gnmi.Get(t, dut, gnmi.OC().System().Cpu(oc.Cpu_Index_ALL).Wait().Max().State())
			if cpuIdle == uint8(0) || cpuIdle > uint8(0) {
				t.Logf("Got correct CPU Wait Max value")
			} else {
				t.Errorf("Unexpected CPU Wait Max value,got: %v want: greater than or equal to %v", cpuIdle, uint8(0))
			}
		})
	})
}

func testCPUWaitInterval(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/wait/interval", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpuIdle := gnmi.Get(t, dut, gnmi.OC().System().Cpu(oc.Cpu_Index_ALL).Wait().Interval().State())
			if cpuIdle == uint64(0) || cpuIdle > uint64(0) {
				t.Logf("Got correct CPU Wait Interval value")
			} else {
				t.Errorf("Unexpected CPU Wait Interval value,got: %v want: greater than or equal to %v", cpuIdle, uint64(0))
			}
		})
	})
}

func testCPUWaitMinTime(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/wait/mintime", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpuIdle := gnmi.Get(t, dut, gnmi.OC().System().Cpu(oc.Cpu_Index_ALL).Wait().MinTime().State())
			if cpuIdle == uint64(0) || cpuIdle > uint64(0) {
				t.Logf("Got correct CPU Wait MinTime value")
			} else {
				t.Errorf("Unexpected CPU Wait MinTime value,got: %v want: greater than or equal to %v", cpuIdle, uint64(0))
			}
		})
	})
}

func testCPUWaitMaxTime(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/wait/maxtime", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpuIdle := gnmi.Get(t, dut, gnmi.OC().System().Cpu(oc.Cpu_Index_ALL).Wait().MaxTime().State())
			if cpuIdle == uint64(0) || cpuIdle > uint64(0) {
				t.Logf("Got correct CPU Wait Maxtime value")
			} else {
				t.Errorf("Unexpected CPU Wait Maxtime value,got: %v want: greater than or equal to %v", cpuIdle, uint64(0))
			}
		})
	})
}

// /system/cpus/cpu/state/hardware-interrupt
func testCPUHardwareInterruptInstant(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/hardware-interrupt/instant", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpuIdle := gnmi.Get(t, dut, gnmi.OC().System().Cpu(oc.Cpu_Index_ALL).HardwareInterrupt().Instant().State())
			if cpuIdle == uint8(0) || cpuIdle > uint8(0) {
				t.Logf("Got correct CPU HardwareInterrupt Instant value")
			} else {
				t.Errorf("Unexpected CPU HardwareInterrupt Instant value,got: %v want: greater than or equal to  %v", cpuIdle, uint8(0))
			}
		})
	})
}

func testCPUHardwareInterruptAvg(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/hardware-interrupt/avg", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpuIdle := gnmi.Get(t, dut, gnmi.OC().System().Cpu(oc.Cpu_Index_ALL).HardwareInterrupt().Avg().State())
			if cpuIdle == uint8(0) || cpuIdle > uint8(0) {
				t.Logf("Got correct CPU HardwareInterrupt Avg value")
			} else {
				t.Errorf("Unexpected CPU HardwareInterrupt Avg value,got: %v want: greater than or equal to %v", cpuIdle, uint8(0))
			}
		})
	})
}

func testCPUHardwareInterruptMin(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/hardware-interrupt/min", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpuIdle := gnmi.Get(t, dut, gnmi.OC().System().Cpu(oc.Cpu_Index_ALL).HardwareInterrupt().Min().State())
			if cpuIdle == uint8(0) || cpuIdle > uint8(0) {
				t.Logf("Got correct CPU HardwareInterrupt Min value")
			} else {
				t.Errorf("Unexpected CPU HardwareInterrupt Min value,got: %v want: greater than or equal to %v", cpuIdle, uint8(0))
			}
		})
	})
}

func testCPUHardwareInterruptMax(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/hardware-interrupt/max", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpuIdle := gnmi.Get(t, dut, gnmi.OC().System().Cpu(oc.Cpu_Index_ALL).HardwareInterrupt().Max().State())
			if cpuIdle == uint8(0) || cpuIdle > uint8(0) {
				t.Logf("Got correct CPU HardwareInterrupt Max value")
			} else {
				t.Errorf("Unexpected CPU HardwareInterrupt Max value,got: %v want: greater than or equal to %v", cpuIdle, uint8(0))
			}
		})
	})
}

func testCPUHardwareInterruptInterval(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/hardware-interrupt/interval", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpuIdle := gnmi.Get(t, dut, gnmi.OC().System().Cpu(oc.Cpu_Index_ALL).HardwareInterrupt().Interval().State())
			if cpuIdle == uint64(0) || cpuIdle > uint64(0) {
				t.Logf("Got correct CPU WHardwareInterruptait Interval value")
			} else {
				t.Errorf("Unexpected CPU HardwareInterrupt Interval value,got: %v want: greater than or equal to %v", cpuIdle, uint64(0))
			}
		})
	})
}

func testCPUHardwareInterruptMinTime(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/hardware-interrupt/mintime", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpuIdle := gnmi.Get(t, dut, gnmi.OC().System().Cpu(oc.Cpu_Index_ALL).HardwareInterrupt().MinTime().State())
			if cpuIdle == uint64(0) || cpuIdle > uint64(0) {
				t.Logf("Got correct CPU HardwareInterrupt MinTime value")
			} else {
				t.Errorf("Unexpected CPU HardwareInterrupt MinTime value,got: %v want: greater than or equal to %v", cpuIdle, uint64(0))
			}
		})
	})
}

func testCPUHardwareInterruptMaxTime(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/hardware-interrupt/maxtime", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpuIdle := gnmi.Get(t, dut, gnmi.OC().System().Cpu(oc.Cpu_Index_ALL).HardwareInterrupt().MaxTime().State())
			if cpuIdle == uint64(0) || cpuIdle > uint64(0) {
				t.Logf("Got correct CPU HardwareInterrupt Maxtime value")
			} else {
				t.Errorf("Unexpected CPU HardwareInterrupt Maxtime value,got: %v want: greater than or equal to %v", cpuIdle, uint64(0))
			}
		})
	})
}

// /system/cpus/cpu/state/software-interrupt
func testCPUSoftwareInterruptInstant(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/software-interrupt/instant", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpuIdle := gnmi.Get(t, dut, gnmi.OC().System().Cpu(oc.Cpu_Index_ALL).SoftwareInterrupt().Instant().State())
			if cpuIdle == uint8(0) || cpuIdle > uint8(0) {
				t.Logf("Got correct CPU SoftwareInterrupt Instant value")
			} else {
				t.Errorf("Unexpected CPU SoftwareInterrupt Instant value,got: %v want: greater than or equal to %v", cpuIdle, uint8(0))
			}
		})
	})
}

func testCPUSoftwareInterruptAvg(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/software-interrupt/avg", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpuIdle := gnmi.Get(t, dut, gnmi.OC().System().Cpu(oc.Cpu_Index_ALL).SoftwareInterrupt().Avg().State())
			if cpuIdle == uint8(0) || cpuIdle > uint8(0) {
				t.Logf("Got correct CPU SoftwareInterrupt Avg value")
			} else {
				t.Errorf("Unexpected CPU SoftwareInterrupt Avg value,got: %v want: greater than or equal to  %v", cpuIdle, uint8(0))
			}
		})
	})
}

func testCPUSoftwareInterruptMin(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/software-interrupt/min", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpuIdle := gnmi.Get(t, dut, gnmi.OC().System().Cpu(oc.Cpu_Index_ALL).SoftwareInterrupt().Min().State())
			if cpuIdle == uint8(0) || cpuIdle > uint8(0) {
				t.Logf("Got correct CPU SoftwareInterrupt Min value")
			} else {
				t.Errorf("Unexpected CPU SoftwareInterrupt Min value,got: %v want: greater than or equal to %v", cpuIdle, uint8(0))
			}
		})
	})
}

func testCPUSoftwareInterruptMax(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/software-interrupt/max", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpuIdle := gnmi.Get(t, dut, gnmi.OC().System().Cpu(oc.Cpu_Index_ALL).SoftwareInterrupt().Max().State())
			if cpuIdle == uint8(0) || cpuIdle > uint8(0) {
				t.Logf("Got correct CPU SoftwareInterrupt Max value")
			} else {
				t.Errorf("Unexpected CPU SoftwareInterrupt Max value,got: %v want: greater than or equal to %v", cpuIdle, uint8(0))
			}
		})
	})
}

func testCPUSoftwareInterruptInterval(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/software-interrupt/interval", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpuIdle := gnmi.Get(t, dut, gnmi.OC().System().Cpu(oc.Cpu_Index_ALL).SoftwareInterrupt().Interval().State())
			if cpuIdle == uint64(0) || cpuIdle > uint64(0) {
				t.Logf("Got correct CPU SoftwareInterrupt Interval value")
			} else {
				t.Errorf("Unexpected CPU SoftwareInterrupt Interval value,got: %v want: greater than or equal to %v", cpuIdle, uint64(0))
			}
		})
	})
}

func testCPUSoftwareInterruptMinTime(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/software-interrupt/mintime", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpuIdle := gnmi.Get(t, dut, gnmi.OC().System().Cpu(oc.Cpu_Index_ALL).SoftwareInterrupt().MinTime().State())
			if cpuIdle == uint64(0) || cpuIdle > uint64(0) {
				t.Logf("Got correct CPU SoftwareInterrupt MinTime value")
			} else {
				t.Errorf("Unexpected CPU SoftwareInterrupt MinTime value,got: %v want: greater than or equal to %v", cpuIdle, uint64(0))
			}
		})
	})
}

func testCPUSoftwareInterruptMaxTime(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Testing /system/cpus/cpu/state/software-interrupt/maxtime", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			cpuIdle := gnmi.Get(t, dut, gnmi.OC().System().Cpu(oc.Cpu_Index_ALL).SoftwareInterrupt().MaxTime().State())
			if cpuIdle == uint64(0) || cpuIdle > uint64(0) {
				t.Logf("Got correct CPU SoftwareInterrupt Maxtime value")
			} else {
				t.Errorf("Unexpected CPU SoftwareInterrupt Maxtime value,got: %v want: greater than or equal to %v", cpuIdle, uint64(0))
			}
		})
	})
}
