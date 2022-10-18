package dieselvk

import vk "github.com/vulkan-go/vulkan"

type CoreBuffer struct {
	buffer        vk.Buffer
	device_memory vk.DeviceMemory
}
