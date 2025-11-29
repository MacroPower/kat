import k8s.api.apps.v1 as appsv1
import manifests

deployment = appsv1.Deployment {
    metadata.name = "nginx-deployment"
    spec: {
        replicas = 3
        selector.matchLabels = {
            app = "nginx"
        }
        template.metadata.labels = {
            app = "nginx"
        }
        template.spec.containers = [
            {
                name = "nginx"
                image = "nginx:1.14.2"
                ports = [
                    {
                        containerPort = 80
                    }
                ]
            }
        ]
    }
}

manifests.yaml_stream([deployment])
