local lighting = import './lighting.libsonnet';

[
  lighting {
    trigger: 'yokai/test/input-a',
    output: 'yokai/test/output',
  },
]
