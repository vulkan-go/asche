package asche

import vk "github.com/vulkan-go/vulkan"

// InstanceExtensions gets a list of instance extensions available on the platform.
func InstanceExtensions() (names []string, err error) {
	defer checkErr(&err)

	var count uint32
	ret := vk.EnumerateInstanceExtensionProperties("", &count, nil)
	orPanic(newError(ret))
	list := make([]vk.ExtensionProperties, count)
	ret = vk.EnumerateInstanceExtensionProperties("", &count, list)
	orPanic(newError(ret))
	for _, ext := range list {
		ext.Deref()
		names = append(names, vk.ToString(ext.ExtensionName[:]))
	}
	return names, err
}

// DeviceExtensions gets a list of instance extensions available on the provided physical device.
func DeviceExtensions(gpu vk.PhysicalDevice) (names []string, err error) {
	defer checkErr(&err)

	var count uint32
	ret := vk.EnumerateDeviceExtensionProperties(gpu, "", &count, nil)
	orPanic(newError(ret))
	list := make([]vk.ExtensionProperties, count)
	ret = vk.EnumerateDeviceExtensionProperties(gpu, "", &count, list)
	orPanic(newError(ret))
	for _, ext := range list {
		ext.Deref()
		names = append(names, vk.ToString(ext.ExtensionName[:]))
	}
	return names, err
}

// ValidationLayers gets a list of validation layers available on the platform.
func ValidationLayers() (names []string, err error) {
	defer checkErr(&err)

	var count uint32
	ret := vk.EnumerateInstanceLayerProperties(&count, nil)
	orPanic(newError(ret))
	list := make([]vk.LayerProperties, count)
	ret = vk.EnumerateInstanceLayerProperties(&count, list)
	orPanic(newError(ret))
	for _, layer := range list {
		layer.Deref()
		names = append(names, vk.ToString(layer.LayerName[:]))
	}
	return names, err
}
