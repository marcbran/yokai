local loadTestCases = import './load_test_cases.libsonnet';

local exampleTests = {
  name: 'examples',
  tests: [
    {
      name: 'count',
      input:: import '../../../examples/count/home_it.libsonnet',
      expected: [
        {
          name: '/simple',
          inputs: [
            { topic: 'yokai/test/input-a', payload: '{"add":1}' },
            { topic: 'yokai/test/input-a', payload: '{"add":3}' },
          ],
          outputs: [
            { topic: 'yokai/test/output', payload: '{"value":1}' },
            { topic: 'yokai/test/output', payload: '{"value":4}' },
          ],
        },
      ],
    },
    {
      name: 'lighting',
      input:: import '../../../examples/lighting/home_it.libsonnet',
      expected: [
        {
          name: '/simple',
          inputs: [
            { topic: 'yokai/test/input-a', payload: '{"action":"double"}' },
          ],
          outputs: [
            { topic: 'yokai/test/output', payload: '{"brightness":127,"state":"on"}' },
          ],
        },
      ],
    },
  ],
};

{
  output(input): loadTestCases(input),
  tests: [
    exampleTests,
  ],
}
