package dieselvk

import vk "github.com/vulkan-go/vulkan"

type BaseInstanceExtensions struct {
	wanted   []string
	required []string
	actual   []string
}

func NewBaseInstanceExtensions(wanted []string, required []string) *BaseInstanceExtensions {
	var base BaseInstanceExtensions
	base.wanted = wanted
	base.required = required
	base.actual, _ = InstanceExtensions()
	return &base
}

func (e *BaseInstanceExtensions) HasRequired() (bool, []string) {
	missing := []string{}

	for _, req := range e.required {
		has := false
		for _, act := range e.actual {
			if req == act {
				has = true
				break
			}
		}
		if !has {
			missing = append(missing, req)
		}
	}

	if len(missing) > 0 {
		return false, missing
	}

	return true, missing
}

func (e *BaseInstanceExtensions) HasWanted() (bool, []string) {
	missing := []string{}

	for _, req := range e.wanted {
		has := false
		for _, act := range e.actual {
			if req == act {
				has = true
				break
			}
		}
		if !has {
			missing = append(missing, req)
		}
	}

	if len(missing) > 0 {
		return false, missing
	}

	return true, missing
}

func (e *BaseInstanceExtensions) GetExtensions() []string {
	implement := []string{}

	for _, req := range e.required {
		implement = append(implement, req)
	}

	for _, want := range e.wanted {
		hasWanted := false
		for _, req := range e.required {
			if want == req {
				hasWanted = true
			}
		}
		if !hasWanted {
			implement = append(implement, want)
		}
	}

	return implement

}

//----------------Device Extensions--------------------//

type BaseDeviceExtensions struct {
	wanted   []string
	required []string
	actual   []string
}

func NewBaseDeviceExtensions(wanted []string, required []string, gpu vk.PhysicalDevice) *BaseDeviceExtensions {
	var base BaseDeviceExtensions
	base.wanted = wanted
	base.required = required
	base.actual, _ = DeviceExtensions(gpu)
	return &base
}

func (e *BaseDeviceExtensions) HasRequired() (bool, []string) {
	missing := []string{}

	for _, req := range e.required {
		has := false
		for _, act := range e.actual {
			if req == act {
				has = true
				break
			}
		}
		if !has {
			missing = append(missing, req)
		}
	}

	if len(missing) > 0 {
		return false, missing
	}

	return true, missing
}

func (e *BaseDeviceExtensions) HasWanted() (bool, []string) {
	missing := []string{}

	for _, req := range e.wanted {
		has := false
		for _, act := range e.actual {
			if req == act {
				has = true
				break
			}
		}
		if !has {
			missing = append(missing, req)
		}
	}

	if len(missing) > 0 {
		return false, missing
	}

	return true, missing
}

func (e *BaseDeviceExtensions) GetExtensions() []string {
	implement := []string{}

	for _, req := range e.required {
		implement = append(implement, req)
	}

	for _, want := range e.wanted {
		hasWanted := false
		for _, req := range e.required {
			if want == req {
				hasWanted = true
			}
		}
		if !hasWanted {
			implement = append(implement, want)
		}
	}

	return implement

}

//----------------Layer Extensions--------------------//

type BaseLayerExtensions struct {
	wanted []string
	actual []string
}

func NewBaseLayerExtensions(wanted []string) *BaseLayerExtensions {
	var base BaseLayerExtensions
	base.wanted = wanted
	base.actual, _ = ValidationLayers()
	return &base
}

//No required layer extensions
func (e *BaseLayerExtensions) HasRequired() (bool, []string) {
	missing := []string{}
	return true, missing
}

func (e *BaseLayerExtensions) HasWanted() (bool, []string) {
	missing := []string{}

	for _, req := range e.wanted {
		has := false
		for _, act := range e.actual {
			if req == act {
				has = true
				break
			}
		}
		if !has {
			missing = append(missing, req)
		}
	}

	if len(missing) > 0 {
		return false, missing
	}

	return true, missing
}

func (e *BaseLayerExtensions) GetExtensions() []string {
	implement := []string{}

	for _, want := range e.wanted {
		implement = append(implement, want)

	}

	return implement

}
