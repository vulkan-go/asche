package dieselvk

import vk "github.com/vulkan-go/vulkan"

type CoreImage struct {

	//Globalized Core Handles. Buffers, Textures, Shaders
	image_views           map[string]vk.ImageView    //Key: (Declared Unique Image View Key) Value: Vulkan Image View
	texture_images        map[string]vk.Image        //Key: (Declared Unique Image Key) Value: Vulkan Image
	texture_device_memory map[string]vk.DeviceMemory //Key: (Declared Unique Image Key) Value Vulkan Device Memory
}
