# DieselVK

### WARNING -- V 0.0.1 BUILD UNDERLYING API IS SUBJECT TO MAJOR REVISION. IF USING THIS LIBARY FOR THE TIME BEING IT IS SUGGESTED AS A FORK ONLY REPOSITORY. I.E ANY USERS SHOULD SIMPLY FORK A LOCAL COPY OF THIS GITHUB REPO. WHEN BUILD IS STABLE WE WILL INLCUDE THE NOTIFICATION. 

-`dieselvk.Core`

- Uses a limited form of json style string dictionary configuration. For main application setup in your main application skeleton ensure the follow string items are set and passed to the `BaseCore` object.

```config := make(map[string]string, 10)
	config["display"] = "true"```

- Other config["example_key"] configurations can be user defined of course and passed to the public functions available through the BaseCore or BaseCore extension implementations.

- Note that most `diesel.vk` access is deemed to be private access only except for the `BaseCore` implementation. Also although certain implicit interfaces may exist...i.e CoreRenderInstasnce & CoreComputeInstance there is no publicly declared interfaces. If you need interface support for objects it is suggested you fork and extend.

- Currently the platforming delineates only between Linux/OSX purely by enforcing that Metal extensions are loaded when the platform is detected as "Darwin". If you need Windows support and additional cross-platforming string paths library should be added on.

- Project is in early stages and is a forked refactor of `vulkan-go/asche`

---------------------

## API Description

`dieselvk.BaseCore`

- GPU entry point and vulkan extension loader. Specifies bounds of application GPU feature requests and underlying API usage. Some  functions may be public where public functions are open for reinterpretation We force GLFW usage and have the core handle window creation and glfw instance handling as well. The core will also enumerate all available queues / command pools / buffers / memory allocation. So core is also a core manager which the instance implementation calls on. The core provides the interface along with the high level /destruction implementation of common vulkan objects. The diesel vk core also is responsible for initialization of core resources (includes queues / command pools / command buffers / devices and core memory. And all objects are held in mapped `map[string]type` objects.

-`dieselvk.CoreRenderInstance`

  -  Paramaterizes basic Vulkan constructs such as desired context of the underlying engine. Tracks multiple physical devices and capabilities and provides access to the desired physical / logical device units / enumerates their capabilities / specifies multi-gpu workloads / specifies compute work group/local group sizing for compute specific instances.

  - The instance is a specific implementation after the core initialization, which is mostly involved with enumerating desired extensions and vulkan wide resources.

-`dieselvk.CoreQueue`- Provides high level queue operation control and manages queue operations. Provides reliable access to reproducible queues, queries their states, and list their properties for the underlying `Core/Instance` implementations

-`dieselvk.CorePool`- Provides high level application pool state tracking and attached command buffers to their own pools. Tracks lifetime of pools, links them to their physical devices and maintains command pools as thread specific objects for submitting work. Core Pools can maintain their own semaphores and tracks the command buffers as submit blocking or not for that specifc instance. Internal buffers can be retrieved and created as needed.

-`dieselvk.CoreDevice`- Maintians linkage of physical device to logical device. List properites and request and tracks device features/memory/capabilities. Delivers core pools and provides device linkage for multi-gpu usage.

-`dieselvk.CoreProgram`- Maintains list of descriptor sets for single shaders and provides high level buffer linkage data to individual shader programs.

-`dieselvk.CoreShader`- Provides a shader set manager for the core instance. List individual shaders and linkages to global buffer data

-`dieselvk.CorePipeline`- Assembles pipeline state info context holding and links with core program objects.

-`dieselvk.CoreSwapchain`- Creates swapchain instance + associated swap chain images. Holds swapchain image view refences. Holds and request Displace surfaces if requested. Manages swap chain fencing and semaphores for frame requests. Provides framebuffer references with default depth + color attachments. Holds desired VSYNC rates if desired.

-`dieselvk.CoreMemory` - Provides GPU/Host memory allocation callbacks and maps allocators to GoLang slices and types

-`dieselvk.CoreBuffers` - Provides High level buffer allocation routines and allows gathering references for shader/pipelines.

-`dieselvk.CoreImage` - Provides high level image allocation, attachment, image view creation, and GPU formatting issues as well as host/gpu 
communication.

-`dieselvk.CoreDisplay` - Manages screen device pixel format and other rendering format issues.
