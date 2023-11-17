docker_build('app', context = '.')

k8s_yaml([
    './tilt/namespace.yaml',
    './tilt/deployment.yaml',
])
