local renderDockerCompose = import './render_docker_compose.libsonnet';

local exampleTests = {
  name: 'examples',
  tests: [
    {
      name: 'count',
      input:: {
        rootDir: '.',
        configFile: 'test.libsonnet',
        version: '1.2.3',
      },
      expected: {
        services: {
          mqtt: {
            image: 'eclipse-mosquitto:latest',
            entrypoint: 'mosquitto',
            command: '-c /mosquitto-no-auth.conf',
          },
          yokai: {
            image: 'ghcr.io/marcbran/yokai:1.2.3',
            depends_on: [
              'mqtt',
            ],
            command: 'run',
            environment: {
              YOKAI_MQTT_BROKER: 'mqtt:1883',
              YOKAI_APP_CONFIG: '/src/test.libsonnet',
            },
            volumes: [
              './.:/src',
            ],
          },
          cli: {
            image: 'hivemq/mqtt-cli',
            command: "sub -h mqtt -t '#' -T",
            depends_on: [
              'mqtt',
            ],
          },
        },
      },
    },
  ],
};

{
  output(input): renderDockerCompose(input.rootDir, input.configFile, input.version),
  tests: [
    exampleTests,
  ],
}
