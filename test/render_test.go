package test

import (
	"log"
	"runtime"
	"testing"

	"github.com/andewx/dieselvk"
	"github.com/go-gl/glfw/v3.3/glfw"
	vk "github.com/vulkan-go/vulkan"
)

const (
	WIDTH  = 500
	HEIGHT = 500
)

func TestRender(t *testing.T) {

	runtime.LockOSThread()
	if err := glfw.Init(); err != nil {
		panic(err)
	}

	defer glfw.Terminate()

	glfw.WindowHint(glfw.Resizable, glfw.True)
	glfw.WindowHint(glfw.Visible, glfw.True)
	glfw.WindowHint(glfw.ClientAPI, glfw.NoAPI)
	vk.SetGetInstanceProcAddr(glfw.GetVulkanGetInstanceProcAddress())

	if err := vk.Init(); err != nil {
		t.Errorf("Unable to initialize application %v", err)
		return
	}

	log.Printf("Creating Vulkan Instance...\n")

	window, errW := glfw.CreateWindow(WIDTH, HEIGHT, "Vulkan", nil, nil)

	if errW != nil {
		panic(errW)
	}

	//Api currently checks some config key/value pairs for creating the render instance and loading in extensions
	config := make(map[string]string, 10)
	config["extensions"] = "default"
	config["display"] = "true"
	config["debug"] = "false"

	vulkan_core := dieselvk.NewBaseCore(config, "default", 5, 5, window)
	vulkan_core.CreateGraphicsInstance("default")
	vk_instance := vulkan_core.GetInstance("default")

	for !window.ShouldClose() {
		vk_instance.Update(0.0)
		glfw.PollEvents()
	}

	vulkan_core.Release()

}
