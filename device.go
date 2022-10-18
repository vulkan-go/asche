package dieselvk

import vk "github.com/vulkan-go/vulkan"

type CoreDevice struct {
	physical_devices                  []vk.PhysicalDevice
	selected_device                   vk.PhysicalDevice
	selected_device_properties        *vk.PhysicalDeviceProperties
	selected_device_memory_properties *vk.PhysicalDeviceMemoryProperties
	handle                            vk.Device
	key                               string
	name                              string
	device_type                       string
	queues                            CoreQueue
	pools                             map[string]vk.CommandPool    //Key: (Unique Device Pool ID) Value: List Command pools (Per thread pool creation)
	command_buffers                   map[string]vk.CommandBuffer  //Key: (Unique Buffer ID) Value: Vulkan Command Buffer Handles
	descriptor_pools                  map[string]vk.DescriptorPool //Key: (Unique Descriptor Pool ID) Value: Vulkan Descriptor Pools
	surface_formats                   map[string]vk.SurfaceFormat  //Key:  (Unique Surface Format ID) Value: Surface Color Format Descriptors
	depth_formats                     map[string]vk.Format         //Key:  (Unique Depth Formats ID) Value: Format
}
