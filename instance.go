package dieselvk

import (
	"fmt"
	"os"

	vk "github.com/vulkan-go/vulkan"
)

const (
	SWAPCHAIN_COUNT     = 3
	MAX_UNIFORM_BUFFERS = 4
)

//Swapchain synchronization
type PerFrame struct {
	pool           *CorePool
	command        []vk.CommandBuffer
	fence          []vk.Fence
	image_acquired []vk.Semaphore
	queue_complete []vk.Semaphore
}

func NewPerFrame(core *CoreRenderInstance) (PerFrame, error) {
	var err error
	m_frame := PerFrame{}

	m_frame.command = make([]vk.CommandBuffer, 1)
	m_frame.fence = make([]vk.Fence, 1)
	m_frame.image_acquired = make([]vk.Semaphore, 1)
	m_frame.queue_complete = make([]vk.Semaphore, 1)
	m_frame.pool, err = NewCorePool(&core.logical_device.handle, core.render_queue_family)

	//Command buffers
	vk.AllocateCommandBuffers(core.logical_device.handle, &vk.CommandBufferAllocateInfo{
		SType:              vk.StructureTypeCommandBufferAllocateInfo,
		CommandPool:        m_frame.pool.pool,
		Level:              vk.CommandBufferLevelPrimary,
		CommandBufferCount: uint32(1),
	}, m_frame.command)

	//Create Fence
	vk.CreateFence(core.logical_device.handle, &vk.FenceCreateInfo{
		SType: vk.StructureTypeFenceCreateInfo,
		PNext: nil,
		Flags: vk.FenceCreateFlags(vk.FenceCreateSignaledBit),
	}, nil, &m_frame.fence[0])

	//Create Semaphores
	vk.CreateSemaphore(core.logical_device.handle, &vk.SemaphoreCreateInfo{
		SType: vk.StructureTypeSemaphoreCreateInfo,
		Flags: vk.SemaphoreCreateFlags(0x00000000),
	}, nil, &m_frame.image_acquired[0])

	//Create Semaphores
	vk.CreateSemaphore(core.logical_device.handle, &vk.SemaphoreCreateInfo{
		SType: vk.StructureTypeSemaphoreCreateInfo,
		Flags: vk.SemaphoreCreateFlags(0x00000000),
	}, nil, &m_frame.queue_complete[0])

	return m_frame, err

}

type CoreRenderInstance struct {

	//Instances
	instance            *vk.Instance
	instance_extensions BaseInstanceExtensions
	device_extensions   BaseDeviceExtensions
	validation_layers   BaseLayerExtensions
	name                string

	//Single Logical Device for the instance
	logical_device      *CoreDevice
	properties          *Usage
	display             *CoreDisplay
	queues              *CoreQueue
	render_queue        *vk.Queue
	render_queue_family uint32

	//Swap chain handles
	swapchain     *CoreSwapchain
	per_frame     []PerFrame
	current_frame int

	//Swapchain Synchronization
	recycled_semaphores []vk.Semaphore

	//Buffers
	uniform_buffers map[string]CoreBuffer

	//Pipelines and renderpasses
	pipelines    *CorePipeline
	renderpasses map[string]*CoreRenderPass

	//Maps program id's to renderpasses & pipelines
	programs map[string]string
	shaders  *CoreShader
}

type CoreComputeInstance struct {

	//Instances
	instance_extensions BaseInstanceExtensions
	device_extensions   BaseDeviceExtensions
	layer_extensions    BaseLayerExtensions

	//Single Logical Device for the instance
	logical_device *CoreDevice
	properties     *Usage

	//Pipelines and renderpasses
	pipelines    map[string]CorePipeline
	renderpasses map[string]CoreRenderPass

	//Maps program id's to renderpasses & pipelines
	programs map[string]string

	//Local Work Groups
	work_group_size  int
	local_group_size int
}

//Creates a new core instance from the given structure and attaches the instance to a primary graphics compatbible device
func NewCoreRenderInstance(instance vk.Instance, name string, instance_exenstions BaseInstanceExtensions, validation_extensions BaseLayerExtensions, device_extensions []string, display *CoreDisplay, shaders *CoreShader) (*CoreRenderInstance, error) {
	var core CoreRenderInstance

	//Core Extensions
	core.instance_extensions = instance_exenstions
	core.validation_layers = validation_extensions

	core.display = display
	core.instance = &instance
	core.logical_device = &CoreDevice{}
	core.logical_device.key = name
	core.name = name
	core.renderpasses = make(map[string]*CoreRenderPass, 4)
	core.programs = make(map[string]string, 4)
	core.shaders = shaders
	core.recycled_semaphores = make([]vk.Semaphore, 0)
	core.uniform_buffers = make(map[string]CoreBuffer, MAX_UNIFORM_BUFFERS)

	if display.surface == nil {
		surfPtr, err := display.window.CreateWindowSurface(instance, nil)
		if err != nil {
			fmt.Printf("Error creating window surface object")
			display.surface = vk.NullSurface
		}
		display.surface = vk.SurfaceFromPointer(surfPtr)
	}

	err := core.Init(device_extensions)
	return &core, err
}

func (core *CoreRenderInstance) Init(device_extensions []string) error {

	var gpu_count uint32
	var gpus []vk.PhysicalDevice

	ret := vk.EnumeratePhysicalDevices(*core.instance, &gpu_count, nil)

	if gpu_count == 0 {
		Fatal(fmt.Errorf("func (core *CoreRenderInstance)Init() -- No valid physical devices found, count is 0\n"))
	}

	gpus = make([]vk.PhysicalDevice, gpu_count)

	ret = vk.EnumeratePhysicalDevices(*core.instance, &gpu_count, gpus)

	if ret != vk.Success {
		Fatal(fmt.Errorf("func (core *CoreRenderInstance)Ini() -- Unable to query physical devices\n"))
	}

	core.logical_device.physical_devices = append(core.logical_device.physical_devices, gpus...)

	//Select Valid Device By Desired Queue Properties
	has_device := false
	for index := 0; index < int(gpu_count); index++ {
		mGPU := gpus[index]
		flag_bits := uint32(vk.QueueGraphicsBit)
		if core.is_valid_device(&mGPU, flag_bits) {
			core.logical_device.selected_device = mGPU
			core.logical_device.selected_device_properties = &vk.PhysicalDeviceProperties{}
			core.logical_device.selected_device_memory_properties = &vk.PhysicalDeviceMemoryProperties{}
			has_device = true
			break
		}
	}

	if !has_device {
		return fmt.Errorf("Could not find suitable GPU device for graphics and presentation\n")
	}

	//Load in extensions
	core.device_extensions = *NewBaseDeviceExtensions(device_extensions, []string{}, core.logical_device.selected_device)

	//Gather device properties
	vk.GetPhysicalDeviceProperties(core.logical_device.selected_device, core.logical_device.selected_device_properties)
	core.logical_device.selected_device_properties.Deref()
	vk.GetPhysicalDeviceMemoryProperties(core.logical_device.selected_device, core.logical_device.selected_device_memory_properties)
	core.logical_device.selected_device_memory_properties.Deref()

	// Select device extensions
	core.device_extensions = *NewBaseDeviceExtensions(core.device_extensions.wanted, []string{}, core.logical_device.selected_device)
	has_extensions, ext_string := core.device_extensions.HasWanted()

	if !has_extensions {
		fmt.Printf("Vulkan Missing Device Extensions %s", ext_string)
	} else {
		fmt.Printf("Vulkan Device Extensions loaded...\n")
	}

	//Bind the suitable device with assigned queues
	device_queue := NewCoreQueue(core.logical_device.selected_device, core.name)
	queue_infos := device_queue.GetCreateInfos()
	dev_extensions := core.device_extensions.GetExtensions()

	//Create Device
	var device vk.Device
	ret = vk.CreateDevice(core.logical_device.selected_device, &vk.DeviceCreateInfo{
		SType:                   vk.StructureTypeDeviceCreateInfo,
		QueueCreateInfoCount:    uint32(len(queue_infos)),
		PQueueCreateInfos:       queue_infos,
		EnabledExtensionCount:   uint32(len(dev_extensions)),
		PpEnabledExtensionNames: safeStrings(dev_extensions),
		EnabledLayerCount:       uint32(len(core.validation_layers.GetExtensions())),
		PpEnabledLayerNames:     safeStrings(core.validation_layers.GetExtensions()),
	}, nil, &device)

	if ret != vk.Success {
		if ret == vk.ErrorFeatureNotPresent || ret == vk.ErrorExtensionNotPresent {
			fmt.Printf("Error certain device features may not be available on the requested GPU device\n%s\nExiting...", dev_extensions)
			os.Exit(1)
		} else {
			fmt.Printf("Fatal error creating device device not found or device state invalid\nExiting...")
			os.Exit(1)
		}
	}

	core.logical_device.handle = device

	device_queue.CreateQueues(device)

	core.queues = device_queue

	found, q_handle, family := device_queue.BindGraphicsQueue(device)

	core.render_queue_family = uint32(family)

	if !found {
		Fatal(fmt.Errorf("No valid queue handle to device\n"))
	}

	core.render_queue = q_handle
	core.swapchain = NewCoreSwapchain(core, SWAPCHAIN_COUNT, core.display)
	core.swapchain.init(core, core.swapchain.depth, core.display)
	core.per_frame = make([]PerFrame, core.swapchain.depth)
	core.renderpasses["Primary"] = NewCoreRenderPass()
	core.renderpasses["Primary"].CreateRenderPass(core, core.display)
	core.swapchain.create_framebuffers(core, &core.renderpasses["Primary"].renderPass)

	dir, err := os.Getwd()

	if err != nil {
		Fatal(err)
	}

	paths := []string{dir + "/shaders/vert.spv", dir + "/shaders/frag.spv"}

	//Shader Modules
	core.shaders.CreateProgram("default", core, paths)

	//Create New Pipleine
	core.pipelines = NewCorePipeline(core)
	pipe_bulder := NewPiplelineBuilder(core, core.shaders.shader_programs["default"])
	core.pipelines.pipelines["default"] = pipe_bulder.BuildPipeline(core, "Primary", core.display, core.pipelines.layouts["default"])

	//Initalize Uniform Buffers
	//core.uniform_buffers["vertex_uniforms"] = NewCoreUniformBuffer(core.logical_device.handle, "vertex_uniforms", 0,
	//	vk.ShaderStageFlags(vk.ShaderStageVertexBit), 4, core.swapchain.depth)

	//Setup Commands
	core.init_per_frame()
	core.setup_commands()

	return nil

}

func (core *CoreRenderInstance) init_per_frame() {
	//Create Commands Per Frame Commands
	var err error
	for index := 0; index < core.swapchain.depth; index++ {
		core.per_frame[index], err = NewPerFrame(core)
	}
	if err != nil {
		Fatal(fmt.Errorf("Could not initiate per frame data\n"))
	}

}

func (core *CoreRenderInstance) destroy_per_frame() {

	//Destroying all per frame data - Warning Vulkan validation will throw an exception
	for index := 0; index < core.swapchain.depth; index++ {
		vk.ResetFences(core.logical_device.handle, uint32(1), core.per_frame[index].fence)
		vk.ResetCommandPool(core.logical_device.handle, core.per_frame[index].pool.pool, vk.CommandPoolResetFlags(vk.CommandPoolResetReleaseResourcesBit))
		vk.DestroySemaphore(core.logical_device.handle, core.per_frame[index].image_acquired[0], nil)
		vk.DestroySemaphore(core.logical_device.handle, core.per_frame[index].queue_complete[0], nil)
	}

	for index := 0; index < len(core.recycled_semaphores); index++ {
		vk.DestroySemaphore(core.logical_device.handle, core.recycled_semaphores[index], nil)
	}

	core.recycled_semaphores = make([]vk.Semaphore, 0)

}

func (core *CoreRenderInstance) destroy_swapchain() {
	core.destroy_per_frame()
	vk.DestroySwapchain(core.logical_device.handle, core.swapchain.swapchain, nil)
}

func (core *CoreRenderInstance) submit_pipeline(image uint32) vk.Result {

	//Pipleline stage flags
	waitDstStageMask := []vk.PipelineStageFlags{
		vk.PipelineStageFlags(vk.PipelineStageColorAttachmentOutputBit),
	}

	submitInfo := vk.SubmitInfo{
		SType:                vk.StructureTypeSubmitInfo,
		WaitSemaphoreCount:   1,
		PWaitSemaphores:      core.per_frame[core.current_frame].image_acquired,
		PWaitDstStageMask:    waitDstStageMask,
		CommandBufferCount:   1,
		PCommandBuffers:      core.per_frame[core.current_frame].command,
		SignalSemaphoreCount: 1,
		PSignalSemaphores:    core.per_frame[core.current_frame].queue_complete,
	}

	queue := core.render_queue

	res_queue := vk.QueueSubmit(*queue, 1, []vk.SubmitInfo{submitInfo}, core.per_frame[core.current_frame].fence[0])

	return res_queue
}

func (core *CoreRenderInstance) Update(delta_time float32) {
	image_index := uint32(0)

	res := core.acquire_next_image(&image_index)

	if res == vk.Suboptimal || res == vk.ErrorOutOfDate {
		core.resize()
		res = core.acquire_next_image(&image_index)
	}

	if res != vk.Success {
		vk.QueueWaitIdle(*core.render_queue)
	}

	core.setup_command(int(core.current_frame), image_index)

	core.submit_pipeline(image_index)

	res = core.present_image(*core.render_queue, image_index)

	if res == vk.ErrorOutOfDate || res == vk.Suboptimal {
		core.resize()
	} else if res != vk.Success {
		Fatal(fmt.Errorf("Failed to present swapchain image\n"))
	}

	core.current_frame = (core.current_frame + 1) % core.swapchain.depth

	return
}

func (core *CoreRenderInstance) present_image(queue vk.Queue, image_index uint32) vk.Result {

	present_info := vk.PresentInfo{}
	present_info.SType = vk.StructureTypePresentInfo
	present_info.WaitSemaphoreCount = 1
	present_info.PWaitSemaphores = []vk.Semaphore{core.per_frame[core.current_frame].queue_complete[0]}
	swaps := []vk.Swapchain{core.swapchain.swapchain}
	present_info.PSwapchains = swaps
	present_info.SwapchainCount = 1
	present_info.PImageIndices = []uint32{image_index}

	return vk.QueuePresent(queue, &present_info)

}

func (core *CoreRenderInstance) release() {
	core.teardown()
	for _, buffer := range core.uniform_buffers {
		vk.DestroyDescriptorSetLayout(core.logical_device.handle, buffer.layout, nil)
	}
}

func (core *CoreRenderInstance) teardown() {
	//TODO destroy framebuffers // destroy semaphores // Destroy Pipline // Destroy Pipline Layout //Destroy

	vk.DeviceWaitIdle(core.logical_device.handle)

	core.swapchain.teardown_framebuffers(core)

	core.destroy_per_frame()

	for _, frame := range core.per_frame {
		vk.DestroyCommandPool(core.logical_device.handle, frame.pool.pool, nil)
	}
	for _, frame := range core.recycled_semaphores {
		vk.DestroySemaphore(core.logical_device.handle, frame, nil)

	}

	for _, pipe := range core.pipelines.pipelines {
		if pipe != vk.NullPipeline {
			vk.DestroyPipeline(core.logical_device.handle, pipe, nil)
		}
	}

	for _, render := range core.renderpasses {
		if render.renderPass != vk.NullRenderPass {
			vk.DestroyRenderPass(core.logical_device.handle, render.renderPass, nil)
		}
	}

	for index, view := range core.swapchain.image_views {
		if view != vk.NullImageView {
			vk.DestroyImageView(core.logical_device.handle, core.swapchain.image_views[index], nil)
		}
	}

	if core.swapchain.swapchain != vk.NullSwapchain {
		vk.DestroySwapchain(core.logical_device.handle, core.swapchain.swapchain, nil)
	}

	if core.swapchain.old_swapchain != vk.NullSwapchain {
		vk.DestroySwapchain(core.logical_device.handle, core.swapchain.old_swapchain, nil)
	}

	if core.display.surface != vk.NullSurface {
		vk.DestroySurface(*core.instance, core.display.surface, nil)
	}

	vk.DestroyDevice(core.logical_device.handle, nil)
}

func (core *CoreRenderInstance) acquire_next_image(image *uint32) vk.Result {

	res := vk.AcquireNextImage(core.logical_device.handle, core.swapchain.swapchain, vk.MaxUint64,
		core.per_frame[core.current_frame].image_acquired[0], nil, image)

	if res != vk.Success {
		//	core.recycled_semaphores = append(core.recycled_semaphores, acquire_semaphore)
		return res
	}

	if core.per_frame[core.current_frame].fence[0] != vk.Fence(vk.NullHandle) {
		vk.WaitForFences(core.logical_device.handle, 1, core.per_frame[core.current_frame].fence, vk.True, vk.MaxUint64)
		vk.ResetFences(core.logical_device.handle, 1, core.per_frame[core.current_frame].fence)
	}

	if core.per_frame[core.current_frame].pool.pool != vk.CommandPool(vk.NullHandle) {
		vk.QueueWaitIdle(*core.render_queue)
		vk.ResetCommandPool(core.logical_device.handle, core.per_frame[core.current_frame].pool.pool, 0)
	}

	return vk.Success

}

func (core *CoreRenderInstance) setup_command(index int, image_index uint32) {

	clearValues := []vk.ClearValue{
		vk.NewClearValue([]float32{0.15, 0.15, 0.15, 1.0}),
		vk.NewClearDepthStencil(1.0, 0.0),
	}

	viewport := vk.Viewport{}
	scissor := vk.Rect2D{}
	viewport.Width = float32(core.swapchain.extent.Width)
	viewport.Height = float32(core.swapchain.extent.Height)
	scissor.Extent.Width = core.swapchain.extent.Width
	scissor.Extent.Height = core.swapchain.extent.Height

	viewports := []vk.Viewport{
		viewport,
	}

	rects := []vk.Rect2D{
		scissor,
	}

	cmd := core.per_frame[index].command
	vk.ResetCommandBuffer(cmd[0], vk.CommandBufferResetFlags(vk.CommandPoolResetReleaseResourcesBit))
	vk.BeginCommandBuffer(cmd[0], &vk.CommandBufferBeginInfo{
		SType: vk.StructureTypeCommandBufferBeginInfo,
		Flags: vk.CommandBufferUsageFlags(vk.CommandBufferUsageOneTimeSubmitBit),
	})

	vk.CmdBeginRenderPass(cmd[0], &vk.RenderPassBeginInfo{
		SType:           vk.StructureTypeRenderPassBeginInfo,
		RenderPass:      core.renderpasses["Primary"].renderPass,
		Framebuffer:     core.swapchain.framebuffers[image_index],
		RenderArea:      core.swapchain.rect,
		ClearValueCount: uint32(len(clearValues)),
		PClearValues:    clearValues,
	}, vk.SubpassContentsInline)

	vk.CmdBindPipeline(cmd[0], vk.PipelineBindPointGraphics, core.pipelines.pipelines["default"])
	vk.CmdSetViewport(cmd[0], 0, 1, viewports)
	vk.CmdSetScissor(cmd[0], 0, 1, rects)
	vk.CmdDraw(cmd[0], 3, 1, 0, 0)

	vk.CmdEndRenderPass(cmd[0])
	vk.EndCommandBuffer(cmd[0])

}

func (core *CoreRenderInstance) setup_commands() {
	// Command Buffer Per Render-Pass per swapchain image which means they are interchangeable
	for i := 0; i < core.swapchain.depth; i++ {
		core.setup_command(i, uint32(i))
	}
}

func (core *CoreRenderInstance) is_valid_device(device *vk.PhysicalDevice, flags uint32) bool {

	q := NewCoreQueue(*device, "Default")
	return q.IsDeviceSuitable(flags)
}

func (core *CoreRenderInstance) resize() {
	var surface_capabilities vk.SurfaceCapabilities
	vk.GetPhysicalDeviceSurfaceCapabilities(core.logical_device.selected_device, core.display.surface, &surface_capabilities)
	surface_capabilities.Deref()

	if surface_capabilities.CurrentExtent.Width == core.swapchain.extent.Width && surface_capabilities.CurrentExtent.Height == core.swapchain.extent.Height {
		return
	}
	core.swapchain.old_swapchain = core.swapchain.swapchain
	vk.DestroySwapchain(core.logical_device.handle, core.swapchain.swapchain, nil)

	if len(core.swapchain.image_views) > 0 {
		for i := 0; i < len(core.swapchain.image_views); i++ {
			vk.DestroyImageView(core.logical_device.handle, core.swapchain.image_views[i], nil)
		}
	}

	core.swapchain.teardown_framebuffers(core)
	core.swapchain.init(core, core.swapchain.depth, core.display)
	vk.DeviceWaitIdle(core.logical_device.handle)
	core.swapchain.create_framebuffers(core, &core.renderpasses["Primary"].renderPass)

}
