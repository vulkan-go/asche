package dieselvk

import (
	"fmt"
	"log"
	"os"
	"runtime"

	vk "github.com/vulkan-go/vulkan"
)

func isError(ret vk.Result) bool {
	return ret != vk.Success
}

func NewError(ret vk.Result) error {
	if ret != vk.Success {
		pc, _, _, ok := runtime.Caller(0)
		if !ok {
			return fmt.Errorf("Vulkan error: %s (%d)",
				vk.Error(ret).Error(), ret)
		}
		frame := newStackFrame(pc)
		return fmt.Errorf("vulkan error: %s (%d) on %s",
			vk.Error(ret).Error(), ret, frame.String())
	}
	return nil
}

func Fatal(err error, finalizers ...func()) {
	if err != nil {
		for _, fn := range finalizers {
			fn()
		}

		file, err := os.OpenFile("fatal_log.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
		if err != nil {
			log.Fatal(err)
		}
		fatal_log := log.New(file, "FATAL: ", log.Ldate|log.Ltime|log.Lshortfile)
		fatal_log.Fatal(err)
	}

}

func checkErr(err *error) {
	if v := recover(); v != nil {
		*err = fmt.Errorf("%+v", v)
	}
}
