package asche

import (
	"errors"

	vk "github.com/vulkan-go/vulkan"
)

type Context interface {
	// OnPlatformUpdate sould be called upon platform update, e.g. when swapchain has been recreated.
	OnPlatformUpdate(platform Platform) error
	// SetOnPrepare sets callback that will be invoked to initialize and prepare application's vulkan state
	// upon context re-init, e.g. when OnPlatformUpdate is called. onCreate could create textures and pipelines,
	// descriptor layouts and render passes.
	SetOnPrepare(onPrepare func(ctx Context) error)
	// SetOnCleanup sets callback that will be invoked to cleanup application's vulkan state
	// upon context re-init, e.g. when OnPlatformUpdate is called. onCreate could destroy textures and pipelines,
	// descriptor layouts and render passes.
	SetOnCleanup(onCleanup func(ctx Context) error)
	// Submit submits a command buffer to the queue.
	// >>>>>>>> Submit(cmd vk.CommandBuffer)
	// SubmitSwapchain submits a command buffer to the queue which renders to the swapchain image.
	// The difference between this and Submit is that extra semaphores might be added to
	// the vk.QueueSubmit call depending on what was passed in to BeginFrame by the platform.
	// >>>>>>>> SubmitSwapchain(cmd vk.CommandBuffer)
	// Device gets the Vulkan device assigned to the context.
	Device() vk.Device
	// Queue gets the Vulkan graphics queue assigned to the context.
	Queue() vk.Queue
	// Platform gets the current platform.
	Platform() Platform
}

type context struct {
	platform  Platform
	device    vk.Device
	onPrepare func(ctx Context) error
	onCleanup func(ctx Context) error

	descPool       vk.DescriptorPool
	cmdPool        vk.CommandPool
	presentCmdPool vk.CommandPool

	swapchain               vk.Swapchain
	swapchainImageResources []*SwapchainImageResources

	textures       []*Texture
	stagingTexture *Texture
	depth          Depth

	cmd            vk.CommandBuffer
	pipelineLayout vk.PipelineLayout
	descLayout     vk.DescriptorSetLayout
	pipelineCache  vk.PipelineCache
	renderPass     vk.RenderPass
	pipeline       vk.Pipeline

	fences []vk.Fence

	imageAcquiredSemaphores  []vk.Semaphore
	drawCompleteSemaphores   []vk.Semaphore
	imageOwnershipSemaphores []vk.Semaphore

	separatePresentQueue bool
	currentBuffer        int
	// queue                vk.Queue
	// swapchainIndex       uint32
	// renderingThreadCount uint
	// perFrameCtxs         []*perFrameCtx
}

// TODO: make  configurable
const frameLag = 2

func (c *context) destroy() {
	c.platform = nil

	// Wait for fences from present operations
	for i := 0; i < len(c.fences); i++ {
		vk.WaitForFences(c.device, 1, []vk.Fence{c.fences[i]}, vk.True, vk.MaxUint64)
		vk.DestroyFence(c.device, c.fences[i], nil)
		vk.DestroySemaphore(c.device, c.imageAcquiredSemaphores[i], nil)
		vk.DestroySemaphore(c.device, c.drawCompleteSemaphores[i], nil)
		if c.separatePresentQueue {
			vk.DestroySemaphore(c.device, c.imageOwnershipSemaphores[i], nil)
		}
	}

	for i := 0; i < len(c.swapchainImageResources); i++ {
		vk.DestroyFramebuffer(c.device, c.swapchainImageResources[i].framebuffer, nil)
	}
	vk.DestroyDescriptorPool(c.device, c.descPool, nil)

	vk.DestroyPipeline(c.device, c.pipeline, nil)
	vk.DestroyPipelineCache(c.device, c.pipelineCache, nil)
	vk.DestroyRenderPass(c.device, c.renderPass, nil)
	vk.DestroyPipelineLayout(c.device, c.pipelineLayout, nil)
	vk.DestroyDescriptorSetLayout(c.device, c.descLayout, nil)

	for i := 0; i < len(c.textures); i++ {
		c.textures[i].Destroy(c.device)
	}
	c.depth.Destroy(c.device)

	for i := 0; i < len(c.swapchainImageResources); i++ {
		c.swapchainImageResources[i].Destroy(c.device, c.cmdPool)
	}
	c.swapchainImageResources = nil
	vk.DestroyCommandPool(c.device, c.cmdPool, nil)
	if c.separatePresentQueue {
		vk.DestroyCommandPool(c.device, c.presentCmdPool, nil)
	}
}

func (c *context) Device() vk.Device {
	return c.device
}

// func (c *context) Queue() vk.Queue {
// 	return c.queue
// }

func (c *context) Platform() Platform {
	return c.platform
}

//     demo_prepare_buffers(demo);
//     demo_prepare_depth(demo);
//     demo_prepare_textures(demo);
//     demo_prepare_cube_data_buffers(demo);
//     demo_prepare_descriptor_layout(demo);
//     demo_prepare_render_pass(demo);
//     demo_prepare_pipeline(demo);
//     demo_prepare_descriptor_pool(demo);
//     demo_prepare_descriptor_set(demo);
//     demo_prepare_framebuffers(demo);
//     for (uint32_t i = 0; i < demo->swapchainImageCount; i++) {
// TODO: take current buffer under control
//         demo->current_buffer = i;
//         demo_draw_build_cmd(demo, demo->swapchain_image_resources[i].cmd);
//     }
// --------------- APP ^^

func (c *context) OnPlatformUpdate(platform Platform) (err error) {
	defer checkErr(&err)
	c.device = platform.Device()
	//	c.queue = platform.Queue()
	c.platform = platform
	vk.DeviceWaitIdle(c.device)

	// vk.DestroyDescriptorPool(c.device, c.descPool, nil)
	// vk.DestroyPipeline(c.device, c.pipeline, nil)
	// vk.DestroyPipelineCache(c.device, c.pipelineCache, nil)
	// vk.DestroyRenderPass(c.device, c.renderPass, nil)
	// vk.DestroyPipelineLayout(c.device, c.pipelineLayout, nil)
	// vk.DestroyDescriptorSetLayout(c.device, c.descLayout, nil)

	// for i := 0; i < len(c.textures); i++ {
	// 	c.textures[i].Destroy(c.device)
	// }
	// c.depth.Destroy(c.device)

	if c.onCleanup != nil {
		orPanic(c.onCleanup(c))
	}
	vk.DestroyCommandPool(c.device, c.cmdPool, nil)
	if c.separatePresentQueue {
		vk.DestroyCommandPool(c.device, c.presentCmdPool, nil)
	}

	ret := vk.CreateCommandPool(c.device, &vk.CommandPoolCreateInfo{
		SType:            vk.StructureTypeCommandPoolCreateInfo,
		QueueFamilyIndex: c.platform.GraphicsQueueFamilyIndex(),
	}, nil, c.cmdPool)
	orPanic(NewError(ret))

	ret = vk.AllocateCommandBuffers(c.device, &vk.CommandBufferAllocateInfo{
		SType:              vk.StructureTypeCommandBufferAllocateInfo,
		CommandPool:        c.cmdPool,
		Level:              vk.CommandBufferLevelPrimary,
		CommandBufferCount: 1,
	}, &c.cmd)
	orPanic(NewError(ret))

	ret = vk.BeginCommandBuffer(c.cmd, &vk.CommandBufferBeginInfo{
		SType: vk.StructureTypeCommandBufferBeginInfo,
	})
	orPanic(NewError(ret))

	//     for (uint32_t i = 0; i < demo->swapchainImageCount; i++) {
	//         err =
	//             vkAllocateCommandBuffers(demo->device, &cmd, &demo->swapchain_image_resources[i].cmd);
	//         assert(!err);
	//     }

	//     if (demo->separate_present_queue) {
	//         const VkCommandPoolCreateInfo present_cmd_pool_info = {
	//             .sType = VK_STRUCTURE_TYPE_COMMAND_POOL_CREATE_INFO,
	//             .pNext = NULL,
	//             .queueFamilyIndex = demo->present_queue_family_index,
	//             .flags = 0,
	//         };
	//         err = vkCreateCommandPool(demo->device, &present_cmd_pool_info, NULL,
	//                                   &demo->present_cmd_pool);
	//         assert(!err);
	//         const VkCommandBufferAllocateInfo present_cmd_info = {
	//             .sType = VK_STRUCTURE_TYPE_COMMAND_BUFFER_ALLOCATE_INFO,
	//             .pNext = NULL,
	//             .commandPool = demo->present_cmd_pool,
	//             .level = VK_COMMAND_BUFFER_LEVEL_PRIMARY,
	//             .commandBufferCount = 1,
	//         };
	//         for (uint32_t i = 0; i < demo->swapchainImageCount; i++) {
	//             err = vkAllocateCommandBuffers(
	//                 demo->device, &present_cmd_info, &demo->swapchain_image_resources[i].graphics_to_present_cmd);
	//             assert(!err);
	//             demo_build_image_ownership_cmd(demo, i);
	//         }
	//     }

	if c.onPrepare != nil {
		orPanic(c.onPrepare(c))
	}
	// Prepare functions above may generate pipeline commands
	// that need to be flushed before beginning the render loop.
	c.flushInitCmd()
	if c.stagingTexture != nil {
		c.stagingTexture.DestroyImage(c.device)
	}
	c.currentBuffer = 0
	return nil
}

func (c *Context) flushInitCmd() {

}

// static void demo_flush_init_cmd(struct demo *demo) {
//     VkResult U_ASSERT_ONLY err;

//     // This function could get called twice if the texture uses a staging buffer
//     // In that case the second call should be ignored
//     if (demo->cmd == VK_NULL_HANDLE)
//         return;

//     err = vkEndCommandBuffer(demo->cmd);
//     assert(!err);

//     VkFence fence;
//     VkFenceCreateInfo fence_ci = {.sType = VK_STRUCTURE_TYPE_FENCE_CREATE_INFO,
//                                   .pNext = NULL,
//                                   .flags = 0};
//     err = vkCreateFence(demo->device, &fence_ci, NULL, &fence);
//     assert(!err);

//     const VkCommandBuffer cmd_bufs[] = {demo->cmd};
//     VkSubmitInfo submit_info = {.sType = VK_STRUCTURE_TYPE_SUBMIT_INFO,
//                                 .pNext = NULL,
//                                 .waitSemaphoreCount = 0,
//                                 .pWaitSemaphores = NULL,
//                                 .pWaitDstStageMask = NULL,
//                                 .commandBufferCount = 1,
//                                 .pCommandBuffers = cmd_bufs,
//                                 .signalSemaphoreCount = 0,
//                                 .pSignalSemaphores = NULL};

//     err = vkQueueSubmit(demo->graphics_queue, 1, &submit_info, fence);
//     assert(!err);

//     err = vkWaitForFences(demo->device, 1, &fence, VK_TRUE, UINT64_MAX);
//     assert(!err);

//     vkFreeCommandBuffers(demo->device, demo->cmd_pool, 1, cmd_bufs);
//     vkDestroyFence(demo->device, fence, NULL);
//     demo->cmd = VK_NULL_HANDLE;
// }

func (c *context) prepareSwapchain(dim *SwapchainDimensions) (*SwapchainDimensions, error) {
	pPhysicalDevice := c.platform.PhysicalDevice()
	pSurface := c.platform.Surface()

	// Read surface capabilities

	var surfaceCapabilities vk.SurfaceCapabilities
	ret := vk.GetPhysicalDeviceSurfaceCapabilities(pPhysicalDevice, pSurface, &surfaceCapabilities)
	orPanic(NewError(ret))
	surfaceCapabilities.Deref()

	// Get available surface pixel formats

	var formatCount uint32
	vk.GetPhysicalDeviceSurfaceFormats(pPhysicalDevice, pSurface, &formatCount, nil)
	formats := make([]vk.SurfaceFormat, formatCount)
	vk.GetPhysicalDeviceSurfaceFormats(pPhysicalDevice, pSurface, &formatCount, formats)

	// Select a proper surface format

	var format vk.SurfaceFormat
	if formatCount == 1 {
		formats[0].Deref()
		if formats[0].Format == vk.FormatUndefined {
			format = formats[0]
			format.Format = dim.Format
		} else {
			format = formats[0]
		}
	} else if formatCount == 0 {
		return dim, errors.New("vulkan error: surface has no pixel formats")
	} else {
		formats[0].Deref()
		// select the first one available
		format = formats[0]
	}

	// Setup swapchain parameters

	var swapchainSize vk.Extent2D
	surfaceCapabilities.CurrentExtent.Deref()
	if surfaceCapabilities.CurrentExtent.Width == vk.MaxUint32 {
		swapchainSize.Width = dim.Width
		swapchainSize.Height = dim.Height
	} else {
		swapchainSize = surfaceCapabilities.CurrentExtent
	}
	// FIFO must be supported by all implementations.
	swapchainPresentMode := vk.PresentModeFifo
	// Determine the number of VkImage's to use in the swapchain.
	// Ideally, we desire to own 1 image at a time, the rest of the images can either be rendered to and/or
	// being queued up for display.
	desiredSwapchainImages := surfaceCapabilities.MinImageCount + 1
	if surfaceCapabilities.MaxImageCount > 0 && desiredSwapchainImages > surfaceCapabilities.MaxImageCount {
		// Application must settle for fewer images than desired.
		desiredSwapchainImages = surfaceCapabilities.MaxImageCount
	}

	// Figure out a suitable surface transform.

	var preTransform vk.SurfaceTransformFlagBits
	requiredTransforms := vk.SurfaceTransformIdentityBit
	supportedTransforms := vk.SurfaceTransformFlagBits(surfaceCapabilities.SupportedTransforms)
	if supportedTransforms&requiredTransforms == requiredTransforms {
		preTransform = requiredTransforms
	} else {
		preTransform = surfaceCapabilities.CurrentTransform
	}

	// Create a swapchain

	var swapchain vk.Swapchain
	oldSwapchain := c.swapchain
	ret = vk.CreateSwapchain(c.device, &vk.SwapchainCreateInfo{
		SType:           vk.StructureTypeSwapchainCreateInfo,
		Surface:         pSurface,
		MinImageCount:   desiredSwapchainImages,
		ImageFormat:     format.Format,
		ImageColorSpace: format.ColorSpace,
		ImageExtent: vk.Extent2D{
			Width:  swapchainSize.Width,
			Height: swapchainSize.Height,
		},
		ImageArrayLayers: 1,
		ImageUsage:       vk.ImageUsageFlags(vk.ImageUsageColorAttachmentBit),
		ImageSharingMode: vk.SharingModeExclusive,
		PreTransform:     preTransform,
		CompositeAlpha:   vk.CompositeAlphaInheritBit,
		PresentMode:      swapchainPresentMode,
		Clipped:          vk.True,
		OldSwapchain:     oldSwapchain,
	}, nil, &swapchain)
	orPanic(NewError(ret))
	if oldSwapchain != vk.NullSwapchain {
		// AMD driver times out waiting on fences used in AcquireNextImage on
		// a swapchain that is subsequently destroyed before the wait.
		vk.WaitForFences(c.device, len(c.fences), c.fences, vk.True, vk.MaxUint64)
		vk.DestroySwapchain(c.device, oldSwapchain, nil)
	}
	c.swapchain = swapchain

	newDimensions := &SwapchainDimensions{
		Width:  swapchainSize.Width,
		Height: swapchainSize.Height,
		Format: format.Format,
	}

	var imageCount uint32
	ret = vk.GetSwapchainImages(c.device, c.swapchain, &imageCount, nil)
	orPanic(NewError(ret))
	swapchainImages := make([]vk.Image, imageCount)
	ret = vk.GetSwapchainImages(c.device, c.swapchain, &imageCount, swapchainImages)
	orPanic(NewError(ret))
	for i := 0; i < len(c.swapchainImageResources); i++ {
		c.swapchainImageResources[i].Destroy(c.device, c.cmdPool)
	}
	c.swapchainImageResources = make([]*SwapchainImageResources, 0, imageCount)
	for i := 0; i < len(swapchainImages); i++ {
		c.swapchainImageResources = append(c.swapchainImageResources, &SwapchainImageResources{
			image: swapchainImages[i],
		})
	}
	return newDimensions, nil
}

type Texture struct {
	sampler vk.Sampler

	image       vk.Image
	imageLayout vk.ImageLayout

	memAlloc vk.MemoryAllocateInfo
	mem      vk.DeviceMemory
	view     vk.ImageView

	texWidth  int32
	texHeight int32
}

func (t *Texture) Destroy(dev vk.Device) {
	vk.DestroyImageView(dev, t.view, nil)
	vk.DestroyImage(dev, t.image, nil)
	vk.FreeMemory(dev, t.mem, nil)
	vk.DestroySampler(dev, t.sampler, nil)
}

func (t *Texture) DestroyImage(dev vk.Device) {
	vk.FreeMemory(dev, t.mem, nil)
	vk.DestroyImage(dev, t.image, nil)
}

type Depth struct {
	format   vk.Format
	image    vk.Image
	memAlloc vk.MemoryAllocateInfo
	mem      vk.DeviceMemory
	view     vk.ImageView
}

func (d *Depth) Destroy(dev vk.Device) {
	vk.DestroyImageView(dev, d.view, nil)
	vk.DestroyImage(dev, d.image, nil)
	vk.FreeMemory(dev, d.mem, nil)
}

type SwapchainImageResources struct {
	image                vk.Image
	cmd                  vk.CommandBuffer
	graphicsToPresentCmd vk.CommandBuffer

	view          vk.ImageView
	framebuffer   vk.Framebuffer
	descriptorSet vk.DescriptorSet

	UniformBuffer vk.Buffer
	UniformMemory vk.DeviceMemory
	Fence         vk.Fence
}

func (s *SwapchainImageResources) Destroy(dev vk.Device, cmdPool ...vk.CommandPool) {
	vk.DestroyFramebuffer(dev, s.framebuffer, nil)
	vk.DestroyImageView(dev, s.view, nil)
	if len(cmdPool) > 0 {
		vk.FreeCommandBuffers(dev, cmdPool[0], 1, []vk.CommandBuffer{
			s.cmd,
		})
	}
	vk.DestroyBuffer(dev, s.UniformBuffer, nil)
	vk.FreeMemory(dev, s.UniformMemory, nil)
	vk.DestroyFence(dev, s.Fence, nil)
}

func (s *SwapchainImageResources) SetImageOwnership() {

}

// void demo_build_image_ownership_cmd(struct demo *demo, int i) {
//     VkResult U_ASSERT_ONLY err;

//     const VkCommandBufferBeginInfo cmd_buf_info = {
//         .sType = VK_STRUCTURE_TYPE_COMMAND_BUFFER_BEGIN_INFO,
//         .pNext = NULL,
//         .flags = VK_COMMAND_BUFFER_USAGE_SIMULTANEOUS_USE_BIT,
//         .pInheritanceInfo = NULL,
//     };
//     err = vkBeginCommandBuffer(demo->swapchain_image_resources[i].graphics_to_present_cmd,
//                                &cmd_buf_info);
//     assert(!err);

//     VkImageMemoryBarrier image_ownership_barrier = {
//         .sType = VK_STRUCTURE_TYPE_IMAGE_MEMORY_BARRIER,
//         .pNext = NULL,
//         .srcAccessMask = 0,
//         .dstAccessMask = VK_ACCESS_COLOR_ATTACHMENT_WRITE_BIT,
//         .oldLayout = VK_IMAGE_LAYOUT_PRESENT_SRC_KHR,
//         .newLayout = VK_IMAGE_LAYOUT_PRESENT_SRC_KHR,
//         .srcQueueFamilyIndex = demo->graphics_queue_family_index,
//         .dstQueueFamilyIndex = demo->present_queue_family_index,
//         .image = demo->swapchain_image_resources[i].image,
//         .subresourceRange = {VK_IMAGE_ASPECT_COLOR_BIT, 0, 1, 0, 1}};

//     vkCmdPipelineBarrier(demo->swapchain_image_resources[i].graphics_to_present_cmd,
//                          VK_PIPELINE_STAGE_COLOR_ATTACHMENT_OUTPUT_BIT,
//                          VK_PIPELINE_STAGE_COLOR_ATTACHMENT_OUTPUT_BIT, 0, 0,
//                          NULL, 0, NULL, 1, &image_ownership_barrier);
//     err = vkEndCommandBuffer(demo->swapchain_image_resources[i].graphics_to_present_cmd);
//     assert(!err);
// }
