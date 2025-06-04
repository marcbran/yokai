{
  config: import './home.jsonnet',
  tests: [
    {
      name: 'simple',
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
}
