package asche

import (
	"fmt"
	"runtime"

	vk "github.com/vulkan-go/vulkan"
)

func isError(ret vk.Result) bool {
	return ret != vk.Success
}

func newError(ret vk.Result) error {
	if ret != vk.Success {
		pc, _, _, ok := runtime.Caller(0)
		if !ok {
			return fmt.Errorf("vulkan error: %d", ret)
		}
		frame := newStackFrame(pc)
		return fmt.Errorf("vulkan error: %d on %s", ret, frame.String())
	}
	return nil
}

func orPanic(err error, finalizers ...func()) {
	if err != nil {
		for _, fn := range finalizers {
			fn()
		}
		panic(err)
	}
}

func checkErr(err *error) {
	if v := recover(); v != nil {
		*err = fmt.Errorf("%+v", v)
	}
}

func checkErrStack(err *error) {
	if v := recover(); v != nil {
		stack := make([]byte, 32*1024)
		n := runtime.Stack(stack, false)
		switch event := v.(type) {
		case error:
			*err = fmt.Errorf("%s\n%s", event.Error(), stack[:n])
		default:
			*err = fmt.Errorf("%+v %s", v, stack[:n])
		}
	}
}
