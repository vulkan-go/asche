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

	config := dieselvk.NewUsage("Vulkan", 5)
	config.String_props["Display"] = "Window"
	map_config := make(map[string]*dieselvk.Usage, 1)
	map_config["Config"] = config

	vulkan_core := dieselvk.NewBaseCore(map_config, []string{"Render"}, "Vulkan App", 5, 5, window)
	vulkan_core.CreateGraphicsInstance("Render")
	vk_instance := vulkan_core.GetInstance("Render")

	for !window.ShouldClose() {
		vk_instance.Update(0.0)
		glfw.PollEvents()
	}

}
