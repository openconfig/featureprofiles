
### Test Module cpu

#### 1. Testing system cpus cpu state index

  Test       | **Testing system cpus cpu state index**
  -|-
  Path       | /system/cpus/cpu/state/index
  Preconditions | DUT should be up and running
  Steps to Execute | 1. Subscribe to `/system/cpus/cpu/state/index`<br>2. Verify the CPU index value
  Expected Result | The CPU index value should be `oc.Cpu_Index_ALL`
  Comments |

#### 2. Testing system cpus cpu state total instant

  Test       | **Testing system cpus cpu state total instant**
  -|-
  Description| This test verifies the CPU state total instant by subscribing to it
  Path       | /system/cpus/cpu/state/total/instant
  Preconditions | DUT should be up and running
  Steps to Execute | 1. Subscribe to `/system/cpus/cpu/state/total/instant`<br>2. Verify the CPU total instant value
  Expected Result | The CPU total instant value should be greater than or equal to `0`
  Comments |

#### 3. Testing system cpus cpu state total avg

  Test       | **Testing system cpus cpu state total avg**
  -|-
  Description| This test verifies the CPU state total average by subscribing to it
  Path       | /system/cpus/cpu/state/total/avg
  Preconditions | DUT should be up and running
  Steps to Execute | 1. Subscribe to `/system/cpus/cpu/state/total/avg`<br>2. Verify the CPU total average value
  Expected Result | The CPU total average value should be greater than or equal to `0`
  Comments |

#### 4. Testing system cpus cpu state total min

  Test       | **Testing system cpus cpu state total min**
  -|-
  Description| This test verifies the CPU state total minimum by subscribing to it
  Path       | /system/cpus/cpu/state/total/min
  Preconditions | DUT should be up and running
  Steps to Execute | 1. Subscribe to `/system/cpus/cpu/state/total/min`<br>2. Verify the CPU total minimum value
  Expected Result | The CPU total minimum value should be greater than or equal to `0`
  Comments |

#### 5. Testing system cpus cpu state total max

  Test       | **Testing system cpus cpu state total max**
  -|-
  Description| This test verifies the CPU state total maximum by subscribing to it
  Path       | /system/cpus/cpu/state/total/max
  Preconditions | DUT should be up and running
  Steps to Execute | 1. Subscribe to `/system/cpus/cpu/state/total/max`<br>2. Verify the CPU total maximum value
  Expected Result | The CPU total maximum value should be greater than or equal to `0`
  Comments |

#### 6. Testing system cpus cpu state total interval

  Test       | **Testing system cpus cpu state total interval**
  -|-
  Description| This test verifies the CPU state total interval by subscribing to it
  Path       | /system/cpus/cpu/state/total/interval
  Preconditions | DUT should be up and running
  Steps to Execute | 1. Subscribe to `/system/cpus/cpu/state/total/interval`<br>2. Verify the CPU total interval value
  Expected Result | The CPU total interval value should be greater than or equal to `0`
  Comments |

#### 7. Testing system cpus cpu state total mintime

  Test       | **Testing system cpus cpu state total mintime**
  -|-
  Description| This test verifies the CPU state total minimum time by subscribing to it
  Path       | /system/cpus/cpu/state/total/mintime
  Preconditions | DUT should be up and running
  Steps to Execute | 1. Subscribe to `/system/cpus/cpu/state/total/mintime`<br>2. Verify the CPU total minimum time value
  Expected Result | The CPU total minimum time value should be greater than or equal to `0`
  Comments |

#### 8. Testing system cpus cpu state total maxtime

  Test       | **Testing system cpus cpu state total maxtime**
  -|-
  Description| This test verifies the CPU state total maximum time by subscribing to it
  Path       | /system/cpus/cpu/state/total/maxtime
  Preconditions | DUT should be up and running
  Steps to Execute | 1. Subscribe to `/system/cpus/cpu/state/total/maxtime`<br>2. Verify the CPU total maximum time value
  Expected Result | The CPU total maximum time value should be greater than or equal to `0`
  Comments |

#### 9. Testing system cpus cpu state user instant

  Test       | **Testing system cpus cpu state user instant**
  -|-
  Description| This test verifies the CPU state user instant by subscribing to it
  Path       | /system/cpus/cpu/state/user/instant
  Preconditions | DUT should be up and running
  Steps to Execute | 1. Subscribe to `/system/cpus/cpu/state/user/instant`<br>2. Verify the CPU user instant value
  Expected Result | The CPU user instant value should be greater than or equal to `0`
  Comments |

#### 10. Testing system cpus cpu state user avg

  Test       | **Testing system cpus cpu state user avg**
  -|-
  Description| This test verifies the CPU state user average by subscribing to it
  Path       | /system/cpus/cpu/state/user/avg
  Preconditions | DUT should be up and running
  Steps to Execute | 1. Subscribe to `/system/cpus/cpu/state/user/avg`<br>2. Verify the CPU user average value
  Expected Result | The CPU user average value should be greater than or equal to `0`
  Comments |

#### 11. Testing system cpus cpu state user min

  Test       | **Testing system cpus cpu state user min**
  -|-
  Description| This test verifies the CPU state user minimum by subscribing to it
  Path       | /system/cpus/cpu/state/user/min
  Preconditions | DUT should be up and running
  Steps to Execute | 1. Subscribe to `/system/cpus/cpu/state/user/min`<br>2. Verify the CPU user minimum value
  Expected Result | The CPU user minimum value should be greater than or equal to `0`
  Comments |

#### 12. Testing system cpus cpu state user max

  Test       | **Testing system cpus cpu state user max**
  -|-
  Description| This test verifies the CPU state user maximum by subscribing to it
  Path       | /system/cpus/cpu/state/user/max
  Preconditions | DUT should be up and running
  Steps to Execute | 1. Subscribe to `/system/cpus/cpu/state/user/max`<br>2. Verify the CPU user maximum value
  Expected Result | The CPU user maximum value should be greater than or equal to `0`
  Comments |

#### 13. Testing system cpus cpu state user interval

  Test       | **Testing system cpus cpu state user interval**
  -|-
  Description| This test verifies the CPU state user interval by subscribing to it
  Path       | /system/cpus/cpu/state/user/interval
  Preconditions | DUT should be up and running
  Steps to Execute | 1. Subscribe to `/system/cpus/cpu/state/user/interval`<br>2. Verify the CPU user interval value
  Expected Result | The CPU user interval value should be greater than or equal to `0`
  Comments |

#### 14. Testing system cpus cpu state user mintime

  Test       | **Testing system cpus cpu state user mintime**
  -|-
  Description| This test verifies the CPU state user minimum time by subscribing to it
  Path       | /system/cpus/cpu/state/user/mintime
  Preconditions | DUT should be up and running
  Steps to Execute | 1. Subscribe to `/system/cpus/cpu/state/user/mintime`<br>2. Verify the CPU user minimum time value
  Expected Result | The CPU user minimum time value should be greater than or equal to `0`
  Comments |

#### 15. Testing system cpus cpu state user maxtime

  Test       | **Testing system cpus cpu state user maxtime**
  -|-
  Description| This test verifies the CPU state user maximum time by subscribing to it
  Path       | /system/cpus/cpu/state/user/maxtime
  Preconditions | DUT should be up and running
  Steps to Execute | 1. Subscribe to `/system/cpus/cpu/state/user/maxtime`<br>2. Verify the CPU user maximum time value
  Expected Result | The CPU user maximum time value should be greater than or equal to `0`
  Comments |

#### 16. Testing system cpus cpu state kernel instant

  Test       | **Testing system cpus cpu state kernel instant**
  -|-
  Description| This test verifies the CPU state kernel instant by subscribing to it
  Path       | /system/cpus/cpu/state/kernel/instant
  Preconditions | DUT should be up and running
  Steps to Execute | 1. Subscribe to `/system/cpus/cpu/state/kernel/instant`<br>2. Verify the CPU kernel instant value
  Expected Result | The CPU kernel instant value should be greater than or equal to `0`
  Comments |

#### 17. Testing system cpus cpu state kernel avg

  Test       | **Testing system cpus cpu state kernel avg**
  -|-
  Description| This test verifies the CPU state kernel average by subscribing to it
  Path       | /system/cpus/cpu/state/kernel/avg
  Preconditions | DUT should be up and running
  Steps to Execute | 1. Subscribe to `/system/cpus/cpu/state/kernel/avg`<br>2. Verify the CPU kernel average value
  Expected Result | The CPU kernel average value should be greater than or equal to `0`
  Comments |

#### 18. Testing system cpus cpu state kernel min

  Test       | **Testing system cpus cpu state kernel min**
  -|-
  Description| This test verifies the CPU state kernel minimum by subscribing to it
  Path       | /system/cpus/cpu/state/kernel/min
  Preconditions | DUT should be up and running
  Steps to Execute | 1. Subscribe to `/system/cpus/cpu/state/kernel/min`<br>2. Verify the CPU kernel minimum value
  Expected Result | The CPU kernel minimum value should be greater than or equal to `0`
  Comments |

#### 19. Testing system cpus cpu state kernel max

  Test       | **Testing system cpus cpu state kernel max**
  -|-
  Description| This test verifies the CPU state kernel maximum by subscribing to it
  Path       | /system/cpus/cpu/state/kernel/max
  Preconditions | DUT should be up and running
  Steps to Execute | 1. Subscribe to `/system/cpus/cpu/state/kernel/max`<br>2. Verify the CPU kernel maximum value
  Expected Result | The CPU kernel maximum value should be greater than or equal to `0`
  Comments |

#### 20. Testing system cpus cpu state kernel interval

  Test       | **Testing system cpus cpu state kernel interval**
  -|-
  Description| This test verifies the CPU state kernel interval by subscribing to it
  Path       | /system/cpus/cpu/state/kernel/interval
  Preconditions | DUT should be up and running
  Steps to Execute | 1. Subscribe to `/system/cpus/cpu/state/kernel/interval`<br>2. Verify the CPU kernel interval value
  Expected Result | The CPU kernel interval value should be greater than or equal to `0`
  Comments |

#### 21. Testing system cpus cpu state kernel mintime

  Test       | **Testing system cpus cpu state kernel mintime**
  -|-
  Description| This test verifies the CPU state kernel minimum time by subscribing to it
  Path       | /system/cpus/cpu/state/kernel/mintime
  Preconditions | DUT should be up and running
  Steps to Execute | 1. Subscribe to `/system/cpus/cpu/state/kernel/mintime`<br>2. Verify the CPU kernel minimum time value
  Expected Result | The CPU kernel minimum time value should be greater than or equal to `0`
  Comments |

#### 22. Testing system cpus cpu state kernel maxtime

  Test       | **Testing system cpus cpu state kernel maxtime**
  -|-
  Description| This test verifies the CPU state kernel maximum time by subscribing to it
  Path       | /system/cpus/cpu/state/kernel/maxtime
  Preconditions | DUT should be up and running
  Steps to Execute | 1. Subscribe to `/system/cpus/cpu/state/kernel/maxtime`<br>2. Verify the CPU kernel maximum time value
  Expected Result | The CPU kernel maximum time value should be greater than or equal to `0`
  Comments |

#### 23. Testing system cpus cpu state nice instant

  Test       | **Testing system cpus cpu state nice instant**
  -|-
  Description| This test verifies the CPU state nice instant by subscribing to it
  Path       | /system/cpus/cpu/state/nice/instant
  Preconditions | DUT should be up and running
  Steps to Execute | 1. Subscribe to `/system/cpus/cpu/state/nice/instant`<br>2. Verify the CPU nice instant value
  Expected Result | The CPU nice instant value should be greater than or equal to `0`
  Comments |

#### 24. Testing system cpus cpu state nice avg

  Test       | **Testing system cpus cpu state nice avg**
  -|-
  Description| This test verifies the CPU state nice average by subscribing to it
  Path       | /system/cpus/cpu/state/nice/avg
  Preconditions | DUT should be up and running
  Steps to Execute | 1. Subscribe to `/system/cpus/cpu/state/nice/avg`<br>2. Verify the CPU nice average value
  Expected Result | The CPU nice average value should be greater than or equal to `0`
  Comments |

#### 25. Testing system cpus cpu state nice min

  Test       | **Testing system cpus cpu state nice min**
  -|-
  Description| This test verifies the CPU state nice minimum by subscribing to it
  Path       | /system/cpus/cpu/state/nice/min
  Preconditions | DUT should be up and running
  Steps to Execute | 1. Subscribe to `/system/cpus/cpu/state/nice/min`<br>2. Verify the CPU nice minimum value
  Expected Result | The CPU nice minimum value should be greater than or equal to `0`
  Comments |

#### 26. Testing system cpus cpu state nice max

  Test       | **Testing system cpus cpu state nice max**
  -|-
  Description| This test verifies the CPU state nice maximum by subscribing to it
  Path       | /system/cpus/cpu/state/nice/max
  Preconditions | DUT should be up and running
  Steps to Execute | 1. Subscribe to `/system/cpus/cpu/state/nice/max`<br>2. Verify the CPU nice maximum value
  Expected Result | The CPU nice maximum value should be greater than or equal to `0`
  Comments |

#### 27. Testing system cpus cpu state nice interval

  Test       | **Testing system cpus cpu state nice interval**
  -|-
  Description| This test verifies the CPU state nice interval by subscribing to it
  Path       | /system/cpus/cpu/state/nice/interval
  Preconditions | DUT should be up and running
  Steps to Execute | 1. Subscribe to `/system/cpus/cpu/state/nice/interval`<br>2. Verify the CPU nice interval value
  Expected Result | The CPU nice interval value should be greater than or equal to `0`
  Comments |

#### 28. Testing system cpus cpu state nice mintime

  Test       | **Testing system cpus cpu state nice mintime**
  -|-
  Description| This test verifies the CPU state nice minimum time by subscribing to it
  Path       | /system/cpus/cpu/state/nice/mintime
  Preconditions | DUT should be up and running
  Steps to Execute | 1. Subscribe to `/system/cpus/cpu/state/nice/mintime`<br>2. Verify the CPU nice minimum time value
  Expected Result | The CPU nice minimum time value should be greater than or equal to `0`
  Comments |

#### 29. Testing system cpus cpu state nice maxtime

  Test       | **Testing system cpus cpu state nice maxtime**
  -|-
  Description| This test verifies the CPU state nice maximum time by subscribing to it
  Path       | /system/cpus/cpu/state/nice/maxtime
  Preconditions | DUT should be up and running
  Steps to Execute | 1. Subscribe to `/system/cpus/cpu/state/nice/maxtime`<br>2. Verify the CPU nice maximum value
  Expected Result | The CPU nice maximum time value should be greater than or equal to `0`
  Comments |

#### 30. Testing system cpus cpu state idle instant

  Test       | **Testing system cpus cpu state idle instant**
  -|-
  Description| This test verifies the CPU state idle instant by subscribing to it
  Path       | /system/cpus/cpu/state/idle/instant
  Preconditions | DUT should be up and running
  Steps to Execute | 1. Subscribe to `/system/cpus/cpu/state/idle/instant`<br>2. Verify the CPU

### Test Module: cpu

#### 31. Testing system cpus cpu state idle avg

  Test       | **Testing system cpus cpu state idle avg**
  -|-
  Description| This test verifies the CPU state idle average by subscribing to it
  Path       | /system/cpus/cpu/state/idle/avg
  Preconditions | DUT should be up and running
  Steps to Execute | 1. Subscribe to `/system/cpus/cpu/state/idle/avg`<br>2. Verify the CPU idle average value
  Expected Result | The CPU idle average value should be greater than or equal to `0`
  Comments |

#### 32. Testing system cpus cpu state idle min

  Test       | **Testing system cpus cpu state idle min**
  -|-
  Description| This test verifies the CPU state idle minimum by subscribing to it
  Path       | /system/cpus/cpu/state/idle/min
  Preconditions | DUT should be up and running
  Steps to Execute | 1. Subscribe to `/system/cpus/cpu/state/idle/min`<br>2. Verify the CPU idle minimum value
  Expected Result | The CPU idle minimum value should be greater than or equal to `0`
  Comments |

#### 33. Testing system cpus cpu state idle max

  Test       | **Testing system cpus cpu state idle max**
  -|-
  Description| This test verifies the CPU state idle maximum by subscribing to it
  Path       | /system/cpus/cpu/state/idle/max
  Preconditions | DUT should be up and running
  Steps to Execute | 1. Subscribe to `/system/cpus/cpu/state/idle/max`<br>2. Verify the CPU idle maximum value
  Expected Result | The CPU idle maximum value should be greater than or equal to `0`
  Comments |

#### 34. Testing system cpus cpu state idle interval

  Test       | **Testing system cpus cpu state idle interval**
  -|-
  Description| This test verifies the CPU state idle interval by subscribing to it
  Path       | /system/cpus/cpu/state/idle/interval
  Preconditions | DUT should be up and running
  Steps to Execute | 1. Subscribe to `/system/cpus/cpu/state/idle/interval`<br>2. Verify the CPU idle interval value
  Expected Result | The CPU idle interval value should be greater than or equal to `0`
  Comments |

#### 35. Testing system cpus cpu state idle mintime

  Test       | **Testing system cpus cpu state idle mintime**
  -|-
  Description| This test verifies the CPU state idle minimum time by subscribing to it
  Path       | /system/cpus/cpu/state/idle/mintime
  Preconditions | DUT should be up and running
  Steps to Execute | 1. Subscribe to `/system/cpus/cpu/state/idle/mintime`<br>2. Verify the CPU idle minimum time value
  Expected Result | The CPU idle minimum time value should be greater than or equal to `0`
  Comments |

#### 36. Testing system cpus cpu state idle maxtime

  Test       | **Testing system cpus cpu state idle maxtime**
  -|-
  Description| This test verifies the CPU state idle maximum time by subscribing to it
  Path       | /system/cpus/cpu/state/idle/maxtime
  Preconditions | DUT should be up and running
  Steps to Execute | 1. Subscribe to `/system/cpus/cpu/state/idle/maxtime`<br>2. Verify the CPU idle maximum time value
  Expected Result | The CPU idle maximum time value should be greater than or equal to `0`
  Comments |

#### 37. Testing system cpus cpu state wait instant

  Test       | **Testing system cpus cpu state wait instant**
  -|-
  Description| This test verifies the CPU state wait instant by subscribing to it
  Path       | /system/cpus/cpu/state/wait/instant
  Preconditions | DUT should be up and running
  Steps to Execute | 1. Subscribe to `/system/cpus/cpu/state/wait/instant`<br>2. Verify the CPU wait instant value
  Expected Result | The CPU wait instant value should be greater than or equal to `0`
  Comments |

#### 38. Testing system cpus cpu state wait avg

  Test       | **Testing system cpus cpu state wait avg**
  -|-
  Description| This test verifies the CPU state wait average by subscribing to it
  Path       | /system/cpus/cpu/state/wait/avg
  Preconditions | DUT should be up and running
  Steps to Execute | 1. Subscribe to `/system/cpus/cpu/state/wait/avg`<br>2. Verify the CPU wait average value
  Expected Result | The CPU wait average value should be greater than or equal to `0`
  Comments |

#### 39. Testing system cpus cpu state wait min

  Test       | **Testing system cpus cpu state wait min**
  -|-
  Description| This test verifies the CPU state wait minimum by subscribing to it
  Path       | /system/cpus/cpu/state/wait/min
  Preconditions | DUT should be up and running
  Steps to Execute | 1. Subscribe to `/system/cpus/cpu/state/wait/min`<br>2. Verify the CPU wait minimum value
  Expected Result | The CPU wait minimum value should be greater than or equal to `0`
  Comments |

#### 40. Testing system cpus cpu state wait max

  Test       | **Testing system cpus cpu state wait max**
  -|-
  Description| This test verifies the CPU state wait maximum by subscribing to it
  Path       | /system/cpus/cpu/state/wait/max
  Preconditions | DUT should be up and running
  Steps to Execute | 1. Subscribe to `/system/cpus/cpu/state/wait/max`<br>2. Verify the CPU wait maximum value
  Expected Result | The CPU wait maximum value should be greater than or equal to `0`
  Comments |

#### 41. Testing system cpus cpu state wait interval

  Test       | **Testing system cpus cpu state wait interval**
  -|-
  Description| This test verifies the CPU state wait interval by subscribing to it
  Path       | /system/cpus/cpu/state/wait/interval
  Preconditions | DUT should be up and running
  Steps to Execute | 1. Subscribe to `/system/cpus/cpu/state/wait/interval`<br>2. Verify the CPU wait interval value
  Expected Result | The CPU wait interval value should be greater than or equal to `0`
  Comments |

#### 42. Testing system cpus cpu state wait mintime

  Test       | **Testing system cpus cpu state wait mintime**
  -|-
  Description| This test verifies the CPU state wait minimum time by subscribing to it
  Path       | /system/cpus/cpu/state/wait/mintime
  Preconditions | DUT should be up and running
  Steps to Execute | 1. Subscribe to `/system/cpus/cpu/state/wait/mintime`<br>2. Verify the CPU wait minimum time value
  Expected Result | The CPU wait minimum time value should be greater than or equal to `0`
  Comments |

#### 43. Testing system cpus cpu state wait maxtime

  Test       | **Testing system cpus cpu state wait maxtime**
  -|-
  Description| This test verifies the CPU state wait maximum time by subscribing to it
  Path       | /system/cpus/cpu/state/wait/maxtime
  Preconditions | DUT should be up and running
  Steps to Execute | 1. Subscribe to `/system/cpus/cpu/state/wait/maxtime`<br>2. Verify the CPU wait maximum time value
  Expected Result | The CPU wait maximum time value should be greater than or equal to `0`
  Comments |

#### 44. Testing system cpus cpu state hardware-interrupt instant

  Test       | **Testing system cpus cpu state hardware-interrupt instant**
  -|-
  Description| This test verifies the CPU state hardware-interrupt instant by subscribing to it
  Path       | /system/cpus/cpu/state/hardware-interrupt/instant
  Preconditions | DUT should be up and running
  Steps to Execute | 1. Subscribe to `/system/cpus/cpu/state/hardware-interrupt/instant`<br>2. Verify the CPU hardware-interrupt instant value
  Expected Result | The CPU hardware-interrupt instant value should be greater than or equal to `0`
  Comments |

#### 45. Testing system cpus cpu state hardware-interrupt avg

  Test       | **Testing system cpus cpu state hardware-interrupt avg**
  -|-
  Description| This test verifies the CPU state hardware-interrupt average by subscribing to it
  Path       | /system/cpus/cpu/state/hardware-interrupt/avg
  Preconditions | DUT should be up and running
  Steps to Execute | 1. Subscribe to `/system/cpus/cpu/state/hardware-interrupt/avg`<br>2. Verify the CPU hardware-interrupt average value
  Expected Result | The CPU hardware-interrupt average value should be greater than or equal to `0`
  Comments |

#### 46. Testing system cpus cpu state hardware-interrupt min

  Test       | **Testing system cpus cpu state hardware-interrupt min**
  -|-
  Description| This test verifies the CPU state hardware-interrupt minimum by subscribing to it
  Path       | /system/cpus/cpu/state/hardware-interrupt/min
  Preconditions | DUT should be up and running
  Steps to Execute | 1. Subscribe to `/system/cpus/cpu/state/hardware-interrupt/min`<br>2. Verify the CPU hardware-interrupt minimum value
  Expected Result | The CPU hardware-interrupt minimum value should be greater than or equal to `0`
  Comments |

#### 47. Testing system cpus cpu state hardware-interrupt max

  Test       | **Testing system cpus cpu state hardware-interrupt max**
  -|-
  Description| This test verifies the CPU state hardware-interrupt maximum by subscribing to it
  Path       | /system/cpus/cpu/state/hardware-interrupt/max
  Preconditions | DUT should be up and running
  Steps to Execute | 1. Subscribe to `/system/cpus/cpu/state/hardware-interrupt/max`<br>2. Verify the CPU hardware-interrupt maximum value
  Expected Result | The CPU hardware-interrupt maximum value should be greater than or equal to `0`
  Comments |

#### 48. Testing system cpus cpu state hardware-interrupt interval

  Test       | **Testing system cpus cpu state hardware-interrupt interval**
  -|-
  Description| This test verifies the CPU state hardware-interrupt interval by subscribing to it
  Path       | /system/cpus/cpu/state/hardware-interrupt/interval
  Preconditions | DUT should be up and running
  Steps to Execute | 1. Subscribe to `/system/cpus/cpu/state/hardware-interrupt/interval`<br>2. Verify the CPU hardware-interrupt interval value
  Expected Result | The CPU hardware-interrupt interval value should be greater than or equal to `0`
  Comments |

#### 49. Testing system cpus cpu state hardware-interrupt mintime

  Test       | **Testing system cpus cpu state hardware-interrupt mintime**
  -|-
  Description| This test verifies the CPU state hardware-interrupt minimum time by subscribing to it
  Path       | /system/cpus/cpu/state/hardware-interrupt/mintime
  Preconditions | DUT should be up and running
  Steps to Execute | 1. Subscribe to `/system/cpus/cpu/state/hardware-interrupt/mintime`<br>2. Verify the CPU hardware-interrupt minimum time value
  Expected Result | The CPU hardware-interrupt minimum time value should be greater than or equal to `0`
  Comments |

#### 50. Testing system cpus cpu state hardware-interrupt maxtime

  Test       | **Testing system cpus cpu state hardware-interrupt maxtime**
  -|-
  Description| This test verifies the CPU state hardware-interrupt maximum time by subscribing to it
  Path       | /system/cpus/cpu/state/hardware-interrupt/maxtime
  Preconditions | DUT should be up and running
  Steps to Execute | 1. Subscribe to `/system/cpus/cpu/state/hardware-interrupt/maxtime`<br>2. Verify the CPU hardware-interrupt maximum time value
  Expected Result | The CPU hardware-interrupt maximum time value should be greater than or equal to `0`
  Comments |

#### 51. Testing system cpus cpu state software-interrupt instant

  Test       | **Testing system cpus cpu state software-interrupt instant**
  -|-
  Description| This test verifies the CPU state software-interrupt instant by subscribing to it
  Path       | /system/cpus/cpu/state/software-interrupt/instant
  Preconditions | DUT should be up and running
  Steps to Execute | 1. Subscribe to `/system/cpus/cpu/state/software-interrupt/instant`<br>2. Verify the CPU software-interrupt instant value
  Expected Result | The CPU software-interrupt instant value should be greater than or equal to `0`
  Comments |

#### 52. Testing system cpus cpu state software-interrupt avg

  Test       | **Testing system cpus cpu state software-interrupt avg**
  -|-
  Description| This test verifies the CPU state software-interrupt average by subscribing to it
  Path       | /system/cpus/cpu/state/software-interrupt/avg
  Preconditions | DUT should be up and running
  Steps to Execute | 1. Subscribe to `/system/cpus/cpu/state/software-interrupt/avg`<br>2. Verify the CPU software-interrupt average value
  Expected Result | The CPU software-interrupt average value should be greater than or equal to `0`
  Comments |

#### 53. Testing system cpus cpu state software-interrupt min

  Test       | **Testing system cpus cpu state software-interrupt min**
  -|-
  Description| This test verifies the CPU state software-interrupt minimum by subscribing to it
  Path       | /system/cpus/cpu/state/software-interrupt/min
  Preconditions | DUT should be up and running
  Steps to Execute | 1. Subscribe to `/system/cpus/cpu/state/software-interrupt/min`<br>2. Verify the CPU software-interrupt minimum value
  Expected Result | The CPU software-interrupt minimum value should be greater than or equal to `0`
  Comments |

#### 54. Testing system cpus cpu state software-interrupt max

  Test       | **Testing system cpus cpu state software-interrupt max**
  -|-
  Description| This test verifies the CPU state software-interrupt maximum by subscribing to it
  Path       | /system/cpus/cpu/state/software-interrupt/max
  Preconditions | DUT should be up and running
  Steps to Execute | 1. Subscribe to `/system/cpus/cpu/state/software-interrupt/max`<br>2. Verify the CPU software-interrupt maximum value
  Expected Result | The CPU software-interrupt maximum value should be greater than or equal to `0`
  Comments |

#### 55. Testing system cpus cpu state software-interrupt interval

  Test       | **Testing system cpus cpu state software-interrupt interval**
  -|-
  Description| This test verifies the CPU state software-interrupt interval by subscribing to it
  Path       | /system/cpus/cpu/state/software-interrupt/interval
  Preconditions | DUT should be up and running
  Steps to Execute | 1. Subscribe to `/system/cpus/cpu/state/software-interrupt/interval`<br>2. Verify the CPU software-interrupt interval value
  Expected Result | The CPU software-interrupt interval value should be greater than or equal to `0`
  Comments |

#### 56. Testing system cpus cpu state software-interrupt mintime

  Test       | **Testing system cpus cpu state software-interrupt mintime**
  -|-
  Description| This test verifies the CPU state software-interrupt minimum time by subscribing to it
  Path       | /system/cpus/cpu/state/software-interrupt/mintime
  Preconditions | DUT should be up and running
  Steps to Execute | 1. Subscribe to `/system/cpus/cpu/state/software-interrupt/mintime`<br>2. Verify the CPU software-interrupt minimum time value
  Expected Result | The CPU software-interrupt minimum time value should be greater than or equal to `0`
  Comments |

#### 57. Testing system cpus cpu state software-interrupt maxtime

  Test       | **Testing system cpus cpu state software-interrupt maxtime**
  -|-
  Description| This test verifies the CPU state software-interrupt maximum time by subscribing to it
  Path       | /system/cpus/cpu/state/software-interrupt/maxtime
  Preconditions | DUT should be up and running
  Steps to Execute | 1. Subscribe to `/system/cpus/cpu/state/software-interrupt/maxtime`<br>2. Verify the CPU software-interrupt maximum time value
  Expected Result | The CPU software-interrupt maximum time value should be greater than or equal to `0`
  Comments |
