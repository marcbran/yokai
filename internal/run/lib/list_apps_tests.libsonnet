local listApps = import './list_apps.libsonnet';

local exampleTests = {
  name: 'examples',
  tests: [
    {
      name: 'count',
      input:: import '../../../examples/count/home.jsonnet',
      expected: {
        count: {
          init: { value: 0 },
          subscriptions: ['yokai/test/input-a'],
        },
      },
    },
    {
      name: 'lighting',
      input:: import '../../../examples/lighting/home.jsonnet',
      expected: {
        lighting: {
          init: null,
          subscriptions: ['yokai/test/input-a'],
        },
      },
    },
  ],
};

{
  output(input): listApps(input),
  tests: [
    exampleTests,
  ],
}
