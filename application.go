package asche

import vk "github.com/vulkan-go/vulkan"

type VulkanMode uint32

const (
	VulkanNone VulkanMode = iota
	VulkanCompute
	VulkanGraphics
	VulkanPresent
)

func (v VulkanMode) Has(mode VulkanMode) bool {
	return v&mode == mode
}

type Application interface {
	VulkanInit(ctx Context) error
	VulkanAPIVersion() vk.Version
	VulkanAppVersion() vk.Version
	VulkanAppName() string
	VulkanMode() VulkanMode
	VulkanSurface() vk.Surface
	VulkanInstanceExtensions() []string
	VulkanDeviceExtensions() []string

	// DECORATORS:
	// ApplicationSwapchainDimensions
	// ApplicationVulkanLayers
}

type ApplicationSwapchainDimensions interface {
	VulkanSwapchainDimensions() SwapchainDimensions
}

type ApplicationVulkanLayers interface {
	VulkanLayers() []string
}

var (
	DefaultVulkanAppVersion = vk.MakeVersion(1, 0, 0)
	DefaultVulkanAPIVersion = vk.MakeVersion(1, 0, 0)
	DefaultVulkanMode       = VulkanCompute | VulkanGraphics | VulkanPresent
)

// SwapchainDimensions describes the size and format of the swapchain.
type SwapchainDimensions struct {
	// Width of the swapchain.
	Width uint32
	// Height of the swapchain.
	Height uint32
	// Format is the pixel format of the swapchain.
	Format vk.Format
}
