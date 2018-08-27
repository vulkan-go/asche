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
	return v&mode != 0
}

type Application interface {
	VulkanInit(ctx Context) error
	VulkanAPIVersion() vk.Version
	VulkanAppVersion() vk.Version
	VulkanAppName() string
	VulkanMode() VulkanMode
	VulkanSurface(instance vk.Instance) vk.Surface
	VulkanInstanceExtensions() []string
	VulkanDeviceExtensions() []string
	VulkanDebug() bool

	// DECORATORS:
	// ApplicationSwapchainDimensions
	// ApplicationVulkanLayers
	// ApplicationContextPrepare
	// ApplicationContextCleanup
	// ApplicationContextInvalidate
}

type ApplicationSwapchainDimensions interface {
	VulkanSwapchainDimensions() *SwapchainDimensions
}

type ApplicationVulkanLayers interface {
	VulkanLayers() []string
}

type ApplicationContextPrepare interface {
	VulkanContextPrepare() error
}

type ApplicationContextCleanup interface {
	VulkanContextCleanup() error
}

type ApplicationContextInvalidate interface {
	VulkanContextInvalidate(imageIdx int) error
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

type BaseVulkanApp struct {
	context Context
}

func (app *BaseVulkanApp) Context() Context {
	return app.context
}

func (app *BaseVulkanApp) VulkanInit(ctx Context) error {
	app.context = ctx
	return nil
}

func (app *BaseVulkanApp) VulkanAPIVersion() vk.Version {
	return vk.Version(vk.MakeVersion(1, 0, 0))
}

func (app *BaseVulkanApp) VulkanAppVersion() vk.Version {
	return vk.Version(vk.MakeVersion(1, 0, 0))
}

func (app *BaseVulkanApp) VulkanAppName() string {
	return "base"
}

func (app *BaseVulkanApp) VulkanMode() VulkanMode {
	return VulkanCompute | VulkanGraphics
}

func (app *BaseVulkanApp) VulkanSurface(instance vk.Instance) vk.Surface {
	return vk.NullSurface
}

func (app *BaseVulkanApp) VulkanInstanceExtensions() []string {
	return nil
}

func (app *BaseVulkanApp) VulkanDeviceExtensions() []string {
	return nil
}

func (app *BaseVulkanApp) VulkanDebug() bool {
	return false
}
