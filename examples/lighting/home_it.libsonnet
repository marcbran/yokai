{
  config: import './home.jsonnet',
  tests: [
    {
      name: 'single',
      inputs: [
        { topic: 'yokai/test/input-a', payload: '{"action":"single"}' },
      ],
      outputs: [
        { topic: 'yokai/test/output', payload: '{"brightness":255,"state":"on"}' },
      ],
    },
    {
      name: 'double',
      inputs: [
        { topic: 'yokai/test/input-a', payload: '{"action":"double"}' },
      ],
      outputs: [
        { topic: 'yokai/test/output', payload: '{"brightness":127,"state":"on"}' },
      ],
    },
  ],
}
