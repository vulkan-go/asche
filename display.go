package dieselvk

import (
	"fmt"

	"github.com/go-gl/glfw/v3.3/glfw"
	vk "github.com/vulkan-go/vulkan"
)

type CoreDisplay struct {
	window         *glfw.Window
	extent         vk.Extent2D
	surface_format vk.SurfaceFormat
	depth_format   vk.Format
	surface        vk.Surface
}

//Creates new core display from window and a logical device
func NewCoreDisplay(window *glfw.Window, instance *vk.Instance) *CoreDisplay {
	var core CoreDisplay
	core.window = window
	return &core
}

func (core *CoreDisplay) GetVulkanSurface(instance *vk.Instance) *vk.Surface {

	ret, err := core.window.CreateWindowSurface(instance, nil)
	if err != nil {
		Fatal(fmt.Errorf("Failed to create vulkan window surface\n"))
	}
	core.surface = vk.SurfaceFromPointer(ret)
	return &core.surface
}

func (core *CoreDisplay) GetSize() (int, int) {
	return core.window.GetSize()
}
