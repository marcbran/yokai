local view = import './view.libsonnet';

local exampleTests = {
  name: 'examples',
  tests: [
    {
      name: 'count',
      input:: {
        config: import '../../../examples/count/home.jsonnet',
        key: 'count',
        model: { value: 3 },
      },
      expected: 'Value: 3',
    },
    {
      name: 'lighting',
      input:: {
        config: import '../../../examples/lighting/home.jsonnet',
        key: 'lighting',
        model: {},
      },
      expected: '',
    },
  ],
};

{
  output(input): view(input.config, input.key, input.model),
  tests: [
    exampleTests,
  ],
}
