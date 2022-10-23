//we will be using glsl version 4.5 syntax
#version 450

layout(location = 0) out vec3 fragColor;

float rand(vec2 co){
    return fract(sin(dot(co, vec2(12.9898, 78.233))) * 43758.5453);
}

void main()
{
	//output the position of each vertex

	//const array of positions for the triangle
const vec3 positions[3] = vec3[3](
	vec3(1.f,1.f, 0.0f),
	vec3(-1.f,1.f, 0.0f),
	vec3(0.f,-1.f, 0.0f)
);

vec3 colors[3] = vec3[](
    vec3(1.0, 0.0, 0.0),
    vec3(0.0, 1.0, 0.0),
    vec3(0.0, 0.0, 1.0)
);

	//randomly adjust the color vectors
	float rand = rand(vec2(0.5,0.5));

	rand = rand;

	colors[0] = vec3(colors[0].x - rand,0,0);
	colors[1] = vec3(0,colors[1].y - rand,0);
	colors[2] = vec3(0,0,colors[2].z - rand);

	gl_Position = vec4(positions[gl_VertexIndex], 1.0f);
    fragColor = colors[gl_VertexIndex];
}