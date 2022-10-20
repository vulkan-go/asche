package dieselvk

import (
	"fmt"
	"os"

	vk "github.com/vulkan-go/vulkan"
)

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
	swapchain        *CoreSwapchain
	framebuffers     []vk.Framebuffer
	swapchain_images []vk.Image
	swapchain_view   []vk.ImageView
	current_buffer   int

	//Swapchain Synchronization
	sem_swapchain_image_acquired []vk.Semaphore
	sem_renderpasses_complete    []vk.Semaphore
	fence_swapchain              []vk.Fence

	//Command Pools and Buffers
	pools    map[string]*CorePool
	commands []vk.CommandBuffer

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
	core.pools = make(map[string]*CorePool, 4)
	core.renderpasses = make(map[string]*CoreRenderPass, 4)
	core.programs = make(map[string]string, 4)
	core.shaders = shaders

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
		if core.IsValidDevice(&mGPU, flag_bits) {
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
	core.CreatePool("Primary")
	core.swapchain = NewCoreSwapchain(core, 3, core.display)
	core.swapchain.Init(core, core.swapchain.depth, core.display)
	core.renderpasses["Primary"] = NewCoreRenderPass()
	core.renderpasses["Primary"].CreateRenderPass(core, core.display)
	core.swapchain.Create_FrameBuffers(core, &core.renderpasses["Primary"].renderPass)

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

	//Setup Commands
	core.Init_Per_Frame()
	core.SetupCommands()

	return nil

}

func (core *CoreRenderInstance) Init_Per_Frame() {
	//Create Commands Per Frame Commands
	core.commands = make([]vk.CommandBuffer, core.swapchain.depth)
	core.fence_swapchain = make([]vk.Fence, core.swapchain.depth)
	core.sem_renderpasses_complete = make([]vk.Semaphore, core.swapchain.depth)
	core.sem_swapchain_image_acquired = make([]vk.Semaphore, core.swapchain.depth)

	vk.AllocateCommandBuffers(core.logical_device.handle, &vk.CommandBufferAllocateInfo{
		SType:              vk.StructureTypeCommandBufferAllocateInfo,
		CommandPool:        core.pools["Primary"].pool,
		Level:              vk.CommandBufferLevelPrimary,
		CommandBufferCount: uint32(core.swapchain.depth),
	}, core.commands)

	for index := 0; index < core.swapchain.depth; index++ {

		//Create Fences
		vk.CreateFence(core.logical_device.handle, &vk.FenceCreateInfo{
			SType: vk.StructureTypeFenceCreateInfo,
			PNext: nil,
			Flags: vk.FenceCreateFlags(vk.FenceCreateSignaledBit),
		}, nil, &core.fence_swapchain[index])

		//Create Semaphores
		vk.CreateSemaphore(core.logical_device.handle, &vk.SemaphoreCreateInfo{
			SType: vk.StructureTypeSemaphoreCreateInfo,
			Flags: vk.SemaphoreCreateFlags(0x00000000),
		}, nil, &core.sem_renderpasses_complete[index])

		//Create Semaphores
		vk.CreateSemaphore(core.logical_device.handle, &vk.SemaphoreCreateInfo{
			SType: vk.StructureTypeSemaphoreCreateInfo,
			Flags: vk.SemaphoreCreateFlags(0x00000000),
		}, nil, &core.sem_swapchain_image_acquired[index])
	}

}

func (core *CoreRenderInstance) Destroy_Per_Frame() {

	vk.ResetCommandPool(core.logical_device.handle, core.pools["Primary"].pool, vk.CommandPoolResetFlags(vk.CommandPoolResetReleaseResourcesBit))
	vk.ResetFences(core.logical_device.handle, uint32(core.swapchain.depth), core.fence_swapchain)

	for index := 0; index < core.swapchain.depth; index++ {
		vk.DestroySemaphore(core.logical_device.handle, core.sem_renderpasses_complete[index], nil)
		vk.DestroySemaphore(core.logical_device.handle, core.sem_swapchain_image_acquired[index], nil)
	}

}

func (core *CoreRenderInstance) Destroy_Swapchain() {
	vk.DestroySwapchain(core.logical_device.handle, core.swapchain.swapchain, nil)
}

func (core *CoreRenderInstance) Release_All() {
	core.Destroy_Per_Frame()
	core.Destroy_Swapchain()
}

func (core *CoreRenderInstance) Update(delta_time float32) {
	image_index := uint32(0)

	//Wait for command buffer fences ....
	fences := []vk.Fence{
		core.fence_swapchain[core.current_buffer],
	}

	//Waits for the fence to be signaled
	vk.WaitForFences(core.logical_device.handle, 1, fences, vk.True, vk.MaxUint64)
	vk.ResetFences(core.logical_device.handle, 1, fences)

	res_image := vk.AcquireNextImage(core.logical_device.handle, core.swapchain.swapchain, vk.MaxUint64, core.sem_swapchain_image_acquired[core.current_buffer], nil, &image_index)

	//Acquire Image Check
	if res_image == vk.ErrorOutOfDate || res_image == vk.Suboptimal {
		core.Resize()
		vk.DeviceWaitIdle(core.logical_device.handle)
		res_image = vk.AcquireNextImage(core.logical_device.handle, core.swapchain.swapchain, vk.MaxUint64,
			core.sem_swapchain_image_acquired[core.current_buffer], nil, &image_index)
		if res_image != vk.Success {
			Fatal(fmt.Errorf("Error could not acquire new swapchain image\n"))
		}
	}

	core.SetupCommand(int(core.current_buffer))

	wait_sems := []vk.Semaphore{core.sem_swapchain_image_acquired[core.current_buffer]}
	signal_sems := []vk.Semaphore{core.sem_renderpasses_complete[core.current_buffer]}

	//Pipleline stage flags
	waitDstStageMask := []vk.PipelineStageFlags{
		vk.PipelineStageFlags(vk.PipelineStageColorAttachmentOutputBit),
	}

	cmd_bufs := []vk.CommandBuffer{
		core.commands[core.current_buffer],
	}

	submitInfo := vk.SubmitInfo{
		SType:                vk.StructureTypeSubmitInfo,
		WaitSemaphoreCount:   1,
		PWaitSemaphores:      wait_sems,
		PWaitDstStageMask:    waitDstStageMask,
		CommandBufferCount:   1,
		PCommandBuffers:      cmd_bufs,
		SignalSemaphoreCount: 1,
		PSignalSemaphores:    signal_sems,
	}

	queue := core.render_queue

	res_queue := vk.QueueSubmit(*queue, 1, []vk.SubmitInfo{submitInfo}, core.fence_swapchain[core.current_buffer])

	if res_queue != vk.Success {
		vk.DeviceWaitIdle(core.logical_device.handle)
		fmt.Printf("Error submitting queue item")
	}

	res_present := core.present_image(*queue, image_index)

	//Present Checks
	if res_present == vk.ErrorOutOfDate || res_present == vk.Suboptimal {
		core.Resize()
		vk.DeviceWaitIdle(core.logical_device.handle)
		res_present = core.present_image(*queue, image_index)
		if res_present != vk.Success {
			Fatal(fmt.Errorf("Error could not acquire new swapchain image\n"))
		}
	}

	core.current_buffer = (core.current_buffer + 1) % (core.swapchain.depth)

	return
}

func (core *CoreRenderInstance) present_image(queue vk.Queue, image_index uint32) vk.Result {

	present_info := vk.PresentInfo{}
	present_info.SType = vk.StructureTypePresentInfo
	present_info.WaitSemaphoreCount = 1
	present_info.PWaitSemaphores = []vk.Semaphore{core.sem_renderpasses_complete[core.current_buffer]}
	swaps := []vk.Swapchain{core.swapchain.swapchain}
	present_info.PSwapchains = swaps
	present_info.SwapchainCount = 1
	present_info.PImageIndices = []uint32{image_index}

	return vk.QueuePresent(queue, &present_info)

}

func (core *CoreRenderInstance) Resize() {
	vk.DeviceWaitIdle(core.logical_device.handle)
	core.swapchain.Teardown_Framebuffers(core)
	core.swapchain.Init(core, core.swapchain.depth, core.display)
	core.renderpasses["Primary"] = NewCoreRenderPass()
	core.renderpasses["Primary"].CreateRenderPass(core, core.display)
	core.swapchain.Create_FrameBuffers(core, &core.renderpasses["Primary"].renderPass)
}

func (core *CoreRenderInstance) Destroy() {

}

func (core *CoreRenderInstance) SetupCommand(index int) {

	clearValues := []vk.ClearValue{
		vk.NewClearValue([]float32{0.0, 0.0, 0.0, 1.0}),
		vk.NewClearDepthStencil(1.0, 0.0),
	}

	cmd := core.commands[index]
	vk.ResetCommandBuffer(cmd, vk.CommandBufferResetFlags(vk.CommandPoolResetReleaseResourcesBit))
	vk.BeginCommandBuffer(cmd, &vk.CommandBufferBeginInfo{
		SType: vk.StructureTypeCommandBufferBeginInfo,
		Flags: vk.CommandBufferUsageFlags(vk.CommandBufferUsageOneTimeSubmitBit),
	})

	vk.CmdBeginRenderPass(cmd, &vk.RenderPassBeginInfo{
		SType:           vk.StructureTypeRenderPassBeginInfo,
		RenderPass:      core.renderpasses["Primary"].renderPass,
		Framebuffer:     core.swapchain.framebuffers[index],
		RenderArea:      core.swapchain.rect,
		ClearValueCount: uint32(len(clearValues)),
		PClearValues:    clearValues,
	}, vk.SubpassContentsInline)

	rects := []vk.Rect2D{
		core.swapchain.rect,
	}
	viewports := []vk.Viewport{
		core.swapchain.viewport,
	}
	vk.CmdSetViewport(cmd, 0, 1, viewports)
	vk.CmdSetScissor(cmd, 0, 1, rects)

	vk.CmdBindPipeline(cmd, vk.PipelineBindPointGraphics, core.pipelines.pipelines["default"])
	vk.CmdDraw(cmd, 3, 1, 0, 0)

	vk.CmdEndRenderPass(cmd)
	vk.EndCommandBuffer(cmd)

}

func (core *CoreRenderInstance) SetupCommands() {
	// Command Buffer Per Render-Pass per swapchain image which means they are interchangeable
	for i := 0; i < len(core.commands); i++ {
		core.SetupCommand(i)
	}
}

func (core *CoreRenderInstance) CreatePool(name string) error {
	core_pool, err := NewCorePool(&core.logical_device.handle, core.render_queue_family)
	if err != nil {
		return err
	}
	core.pools[name] = core_pool
	return nil
}

func (core *CoreRenderInstance) IsValidDevice(device *vk.PhysicalDevice, flags uint32) bool {

	q := NewCoreQueue(*device, "Default")
	return q.IsDeviceSuitable(flags)
}
