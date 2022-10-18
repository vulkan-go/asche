package dieselvk

import (
	"fmt"

	vk "github.com/vulkan-go/vulkan"
)

type CoreSwapchain struct {
	display       *CoreDisplay
	depth         int
	swapchain     *vk.Swapchain
	framebuffers  []vk.Framebuffer
	extent        vk.Extent2D
	rect          vk.Rect2D
	old_swapchain *vk.Swapchain
	images        []vk.Image
	image_views   []vk.ImageView
	viewport      vk.Viewport
}

//Initializes a new core swapchain which sets further display properties, since for right now displays
//are a shared feature be careful to not attach multiple swapchains to a display
func NewCoreSwapchain(instance *CoreRenderInstance, desired_depth int, display *CoreDisplay) *CoreSwapchain {

	var core CoreSwapchain

	core.display = display
	core.depth = desired_depth
	surface := display.surface

	var surfaceCapabilities vk.SurfaceCapabilities
	ret := vk.GetPhysicalDeviceSurfaceCapabilities(instance.logical_device.selected_device, surface, &surfaceCapabilities)
	surfaceCapabilities.Deref()

	// Get available surface pixel formats
	var formatCount uint32
	vk.GetPhysicalDeviceSurfaceFormats(instance.logical_device.selected_device, surface, &formatCount, nil)
	formats := make([]vk.SurfaceFormat, formatCount)
	vk.GetPhysicalDeviceSurfaceFormats(instance.logical_device.selected_device, surface, &formatCount, formats)

	//Select an available format or go with default sRGBA
	var format vk.SurfaceFormat
	if formatCount >= 1 {
		formats[0].Deref()
		if formats[0].Format == vk.FormatUndefined {
			format = formats[0]
			format.Format = vk.FormatA8b8g8r8SrgbPack32
		} else {
			format = formats[0]
		}
	} else {
		Fatal(fmt.Errorf("No suitable surface color format found for display\n"))
	}

	display.surface_format = format

	// Since all depth formats may be optional, we need to find a suitable depth format to use
	// Start with the highest precision packed format
	depthFormats := []vk.Format{
		vk.FormatD32SfloatS8Uint,
		vk.FormatD32Sfloat,
		vk.FormatD24UnormS8Uint,
		vk.FormatD16UnormS8Uint,
		vk.FormatD16Unorm,
	}

	//Hardcoding
	display.depth_format = depthFormats[1]

	//Match swapchain extent to the surface capabilities
	var swapchainSize vk.Extent2D
	surfaceCapabilities.CurrentExtent.Deref()
	if surfaceCapabilities.CurrentExtent.Width == vk.MaxUint32 {
		Fatal(fmt.Errorf("Surface capabilities return invalid frame width\n"))
	} else {
		swapchainSize = surfaceCapabilities.CurrentExtent
	}

	//left, top, right, bottom := glfw.GetCurrentContext().GetFrameSize()
	core.extent = swapchainSize

	core.rect = vk.Rect2D{
		Offset: vk.Offset2D{},
		Extent: core.extent,
	}

	core.display.extent = core.extent

	// The FIFO present mode is guaranteed by the spec to be supported
	swapchainPresentMode := vk.PresentModeFifo

	// Determine the number of VkImage's to use in the swapchain.
	desiredSwapchainImages := uint32(desired_depth)

	if surfaceCapabilities.MaxImageCount > 0 && desiredSwapchainImages > surfaceCapabilities.MaxImageCount {
		desiredSwapchainImages = surfaceCapabilities.MaxImageCount
	} else if desiredSwapchainImages < surfaceCapabilities.MinImageCount {
		desiredSwapchainImages = surfaceCapabilities.MinImageCount
	}

	core.depth = int(desiredSwapchainImages)
	core.images = make([]vk.Image, core.depth)
	core.image_views = make([]vk.ImageView, core.depth)
	core.framebuffers = make([]vk.Framebuffer, core.depth) //framebuffers attach to the swapchain images and create additional depth buffers etc..

	// Figure out a suitable surface transform.
	var preTransform vk.SurfaceTransformFlagBits
	requiredTransforms := vk.SurfaceTransformIdentityBit
	supportedTransforms := surfaceCapabilities.SupportedTransforms
	if vk.SurfaceTransformFlagBits(supportedTransforms)&requiredTransforms != 0 {
		preTransform = requiredTransforms
	} else {
		preTransform = surfaceCapabilities.CurrentTransform
	}

	// Find a supported composite alpha mode - one of these is guaranteed to be set
	compositeAlpha := vk.CompositeAlphaOpaqueBit
	compositeAlphaFlags := []vk.CompositeAlphaFlagBits{
		vk.CompositeAlphaOpaqueBit,
		vk.CompositeAlphaPreMultipliedBit,
		vk.CompositeAlphaPostMultipliedBit,
		vk.CompositeAlphaInheritBit,
	}
	for i := 0; i < len(compositeAlphaFlags); i++ {
		alphaFlags := vk.CompositeAlphaFlags(compositeAlphaFlags[i])
		flagSupported := surfaceCapabilities.SupportedCompositeAlpha&alphaFlags != 0
		if flagSupported {
			compositeAlpha = compositeAlphaFlags[i]
			break
		}
	}

	// Create a swapchain
	var swapchain vk.Swapchain
	core.swapchain = &swapchain
	core.old_swapchain = core.swapchain

	ret = vk.CreateSwapchain(instance.logical_device.handle, &vk.SwapchainCreateInfo{
		SType:           vk.StructureTypeSwapchainCreateInfo,
		Surface:         surface,
		MinImageCount:   uint32(core.depth),
		ImageFormat:     format.Format,
		ImageColorSpace: format.ColorSpace,
		ImageExtent: vk.Extent2D{
			Width:  core.rect.Extent.Width,
			Height: core.rect.Extent.Height,
		},
		ImageUsage:       vk.ImageUsageFlags(vk.ImageUsageColorAttachmentBit),
		PreTransform:     preTransform,
		CompositeAlpha:   compositeAlpha,
		ImageArrayLayers: 1,
		ImageSharingMode: vk.SharingModeExclusive,
		PresentMode:      swapchainPresentMode,
		OldSwapchain:     *core.old_swapchain,
		Clipped:          vk.True,
	}, nil, &swapchain)
	Fatal(NewError(ret))

	if *core.old_swapchain != vk.NullSwapchain {
		vk.DestroySwapchain(instance.logical_device.handle, *core.old_swapchain, nil)
	}

	core.swapchain = &swapchain

	//Creates handles for the swapchain images
	var imageCount uint32
	ret = vk.GetSwapchainImages(instance.logical_device.handle, *core.swapchain, &imageCount, nil)
	core.images = make([]vk.Image, desiredSwapchainImages)
	ret = vk.GetSwapchainImages(instance.logical_device.handle, *core.swapchain, &imageCount, core.images)
	core.image_views = make([]vk.ImageView, imageCount)

	for index := uint32(0); index < imageCount; index++ {
		core.CreateFrameImageView(int(index), instance, &core.images[index])
	}

	//Viewports
	core.viewport = vk.Viewport{
		X:        0.0,
		Y:        1.0,
		Width:    float32(core.rect.Extent.Width),
		Height:   float32(core.rect.Extent.Height),
		MinDepth: 0.0,
		MaxDepth: 1.0,
	}

	core.display.viewport = core.viewport

	return &core
}

func (core *CoreSwapchain) CreateFrameImageView(index int, instance *CoreRenderInstance, m_image_handle *vk.Image) {

	var m_image_view vk.ImageView

	vk.CreateImageView(instance.logical_device.handle,
		&vk.ImageViewCreateInfo{
			SType:    vk.StructureTypeImageViewCreateInfo,
			Flags:    vk.ImageViewCreateFlags(0),
			Image:    *m_image_handle,
			ViewType: vk.ImageViewType2d,
			Format:   core.display.surface_format.Format,
			Components: vk.ComponentMapping{
				R: vk.ComponentSwizzleR,
				G: vk.ComponentSwizzleG,
				B: vk.ComponentSwizzleB,
				A: vk.ComponentSwizzleA,
			},
			SubresourceRange: vk.ImageSubresourceRange{
				AspectMask: vk.ImageAspectFlags(vk.ImageAspectColorBit),
				LevelCount: 1,
				LayerCount: 1,
			}}, nil, &m_image_view)

	core.image_views[index] = m_image_view

}

func (core *CoreSwapchain) CreateFrameBuffer(instance *CoreRenderInstance, renderpass *vk.RenderPass) {

	var depthImage vk.Image
	queue_fam := []uint32{uint32(instance.render_queue_family)}
	res := vk.CreateImage(instance.logical_device.handle, &vk.ImageCreateInfo{
		SType:                 vk.StructureTypeImageCreateInfo,
		Flags:                 vk.ImageCreateFlags(vk.ImageCreateMutableFormatBit),
		ImageType:             vk.ImageType2d,
		Format:                core.display.surface_format.Format,
		Extent:                vk.Extent3D{Width: core.extent.Width, Height: core.extent.Height, Depth: 1},
		MipLevels:             1,
		ArrayLayers:           1,
		Samples:               vk.SampleCount1Bit,
		Tiling:                vk.ImageTilingOptimal,
		Usage:                 vk.ImageUsageFlags(vk.ImageUsageDepthStencilAttachmentBit),
		SharingMode:           vk.SharingModeExclusive,
		QueueFamilyIndexCount: 1,
		PQueueFamilyIndices:   queue_fam,
		InitialLayout:         vk.ImageLayoutUndefined,
	}, nil, &depthImage)

	if res != vk.Success {
		Fatal(NewError(res))
	}

	//Search through GPU memory properties to see if this can be device local
	var depth_memory_req vk.MemoryRequirements
	vk.GetImageMemoryRequirements(instance.logical_device.handle, depthImage, &depth_memory_req)
	depth_memory_req.Deref()

	mem_type_index, _ := vk.FindMemoryTypeIndex(instance.logical_device.selected_device, depth_memory_req.MemoryTypeBits,
		vk.MemoryPropertyFlagBits(vk.MemoryHeapDeviceLocalBit))

	alloc_info := vk.MemoryAllocateInfo{
		SType:           vk.StructureTypeMemoryAllocateInfo,
		AllocationSize:  depth_memory_req.Size,
		MemoryTypeIndex: mem_type_index,
	}

	var depth_memory vk.DeviceMemory

	res = vk.AllocateMemory(instance.logical_device.handle, &alloc_info, nil, &depth_memory)

	if res != vk.Success {
		Fatal(NewError(res))
	}

	vk.BindImageMemory(instance.logical_device.handle, depthImage, depth_memory, 0)

	var depth_image_view vk.ImageView

	res = vk.CreateImageView(instance.logical_device.handle,
		&vk.ImageViewCreateInfo{
			SType:    vk.StructureTypeImageViewCreateInfo,
			Flags:    vk.ImageViewCreateFlags(0),
			Image:    depthImage,
			ViewType: vk.ImageViewType2d,
			Format:   core.display.surface_format.Format,
			Components: vk.ComponentMapping{
				R: vk.ComponentSwizzleR,
				G: vk.ComponentSwizzleG,
				B: vk.ComponentSwizzleB,
				A: vk.ComponentSwizzleA,
			},
			SubresourceRange: vk.ImageSubresourceRange{
				AspectMask: vk.ImageAspectFlags(vk.ImageAspectDepthBit),
				LevelCount: 1,
				LayerCount: 1,
			}}, nil, &depth_image_view)

	if res != vk.Success {
		Fatal(NewError(res))
	}

	for index := 0; index < len(core.images); index++ {

		var framebuffer vk.Framebuffer
		views := []vk.ImageView{core.image_views[index], depth_image_view}
		res = vk.CreateFramebuffer(instance.logical_device.handle, &vk.FramebufferCreateInfo{
			SType:           vk.StructureTypeFramebufferCreateInfo,
			Flags:           vk.FramebufferCreateFlags(0),
			RenderPass:      *renderpass,
			AttachmentCount: uint32(len(views)),
			PAttachments:    views,
			Width:           core.extent.Width,
			Height:          core.extent.Height,
			Layers:          1,
		}, nil, &framebuffer)

		core.framebuffers[index] = framebuffer

		if res != vk.Success {
			Fatal(NewError(res))
		}
	}
}
