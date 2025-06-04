local update = import './update.libsonnet';

local exampleTests = {
  name: 'examples',
  tests: [
    {
      name: 'count',
      input:: {
        config: import '../../../examples/count/home.jsonnet',
        key: 'count',
        topic: 'yokai/test/input-a',
        payload: { add: 1 },
        model: { value: 0 },
      },
      expected: {
        model: { value: 1 },
        'yokai/test/output': { value: 1 },
      },
    },
  ],
};

{
  output(input): update(input.config, input.key, input.topic, input.payload, input.model),
  tests: [
    exampleTests,
  ],
}
