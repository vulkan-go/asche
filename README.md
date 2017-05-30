## Asche

<img src="/docs/asche-logo.png" width="300" />

_...because when you throw a gopher into volcano you get a pile of ash._

Asche is a high-level framework created to simplify development of Vulkan API applications using Go programming language. It manages Vulkan platform state and initialization, also provides an interface the app must conform in order to tell about desired platform requirements.

Currently it's used in [VulkanCube](https://github.com/vulkan-go/demos/blob/master/vulkancube/vulkancube_android/main.go) demo app, please reference to it as an official Asche reference for now.

You should start by implementing this platform-describing interface:

```golang
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
```

Usually the cross-platform code may inherit (by embedding) the default app `as.BaseVulkanApp`, and later you wrap your cross-platform code into platform-specific (as shown in Android and Desktop demos of cube) overriding method implementations for that interface. `VulkanInit()` may stay here from default app, as it doesn't do anything other than storing context state. `ApplicationContextPrepare()` can be overriden in cross-platform code as it must implement the application-specific logic. And finally the platform code overrides `VulkanSurface()` as it should acquire a valid `vk.Instance` using **platform-specific** `vk.CreateWindowSurface`.

Decorators are considered to be optional methods that will be checked in runtime, with no default implementation, must be provided when needed by the app logic.

After platform intialization using `as.NewPlatform`, the application has access to this Vulkan Platform Interface:

```golang
type Platform interface {
    // MemoryProperties gets the current Vulkan physical device memory properties.
    MemoryProperties() vk.PhysicalDeviceMemoryProperties
    // PhysicalDeviceProperies gets the current Vulkan physical device properties.
    PhysicalDeviceProperies() vk.PhysicalDeviceProperties
    // GraphicsQueueFamilyIndex gets the current Vulkan graphics queue family index.
    GraphicsQueueFamilyIndex() uint32
    // PresentQueueFamilyIndex gets the current Vulkan present queue family index.
    PresentQueueFamilyIndex() uint32
    // HasSeparatePresentQueue is true when PresentQueueFamilyIndex differs from GraphicsQueueFamilyIndex.
    HasSeparatePresentQueue() bool
    // GraphicsQueue gets the current Vulkan graphics queue.
    GraphicsQueue() vk.Queue
    // PresentQueue gets the current Vulkan present queue.
    PresentQueue() vk.Queue
    // Instance gets the current Vulkan instance.
    Instance() vk.Instance
    // Device gets the current Vulkan device.
    Device() vk.Device
    // PhysicalDevice gets the current Vulkan physical device.
    PhysicalDevice() vk.PhysicalDevice
    // Surface gets the current Vulkan surface.
    Surface() vk.Surface
    // Destroy is the destructor for the Platform instance.
    Destroy()
}
```

And of course the Vulkan Context that can should be used in the app's logic and rendering loop:

```golang
type Context interface {
    // SetOnPrepare sets callback that will be invoked to initialize and prepare application's vulkan state
    // upon context prepare step. onCreate could create textures and pipelines,
    // descriptor layouts and render passes.
    SetOnPrepare(onPrepare func() error)
    // SetOnCleanup sets callback that will be invoked to cleanup application's vulkan state
    // upon context prepare step. onCreate could destroy textures and pipelines,
    // descriptor layouts and render passes.
    SetOnCleanup(onCleanup func() error)
    // SetOnInvalidate sets callback that will be invoked when context has been invalidated,
    // the application must update its state and prepare the corresponding swapchain image to be presented.
    // onInvalidate could compute new vertex and color data in swapchain image resource buffers.
    SetOnInvalidate(onInvalidate func(imageIdx int) error)
    // Device gets the Vulkan device assigned to the context.
    Device() vk.Device
    // Platform gets the current platform.
    Platform() Platform
    // CommandBuffer gets a command buffer currently active.
    CommandBuffer() vk.CommandBuffer
    // SwapchainDimensions gets the current swapchain dimensions, including pixel format.
    SwapchainDimensions() *SwapchainDimensions
    // SwapchainImageResources exposes the swapchain initialized image resources.
    SwapchainImageResources() []*SwapchainImageResources
    // AcquireNextImage
    AcquireNextImage() (imageIndex int, outdated bool, err error)
    // PresentImage
    PresentImage(imageIdx int) (outdated bool, err error)
}
```

Both **Vulkan Platform Interface** and **Vulkan Context** terms are made up just for clarity, please note that Vulkan API has a little to none amount of abstraction, so Asche provides this state management tools to free the developer from extra burden. However, it's too easy to create leaky abstractions for Vulkan API, so Asche tries to be as minimal and pragmatic as possible.

## License

MIT
