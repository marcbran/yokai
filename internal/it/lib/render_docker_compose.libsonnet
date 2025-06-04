local renderDockerCompose(rootDir, configFile, version) =
  local data = {
    rootDir: rootDir,
    configFile: configFile,
    version: version,
  };
  {
    services: {
      mqtt: {
        image: 'eclipse-mosquitto:latest',
        entrypoint: 'mosquitto',
        command: '-c /mosquitto-no-auth.conf',
      },
      yokai: {
        image: 'ghcr.io/marcbran/yokai:%(version)s' % data,
        depends_on: [
          'mqtt',
        ],
        command: 'run',
        environment: {
          YOKAI_BROKER: 'mqtt:1883',
          YOKAI_APP_CONFIG: '/src/%(configFile)s' % data,
        },
        volumes: [
          './%(rootDir)s:/src' % data,
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
  };

renderDockerCompose
