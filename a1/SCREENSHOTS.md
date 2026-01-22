# Screenshots

## GKE Cluster and Workload Deployment

| Screenshot | Screenshot |
|------------|------------|
| ![Running Cluster](./screenshots/running-cluster.png) | ![Running Workloads](./screenshots/running-workloads.png) |
| **Running GKE Cluster**<br/>Shows the active GKE Autopilot cluster used for deploying the airport security system microservices. | **Running Workloads**<br/>Displays all deployed workloads (Deployments) running in the GKE cluster, including Collector, Camera, Analysis, Recognition, Section, and Alert services. |

## Service Deployments in GKE Console

| Screenshot | Screenshot |
|------------|------------|
| ![Collector GKE](./screenshots/collector-gke.png) | ![Camera GKE](./screenshots/camera-gke.png) |
| **Collector Service Deployment**<br/>Details of the Collector service deployment, showing pod status, replicas, and resource allocation. | **Camera Service Deployment**<br/>Camera service deployment configuration in GKE, managing the simulated surveillance camera feeds. |
| ![Recognition GKE](./screenshots/recognition-gke.png) | ![Analysis GKE](./screenshots/analysis-gke.png) |
| **Face Recognition Service**<br/>Recognition service deployment showing the face recognition microservice that identifies persons of interest. | **Image Analysis Service**<br/>Analysis service deployment handling age and gender estimation from camera frames. |
| ![Section GKE](./screenshots/section-gke.png) | ![Alert GKE](./screenshots/alert-gke.png) |
| **Section Service Deployment**<br/>Section service managing demographic statistics storage with MongoDB integration. | **Alert Service Deployment**<br/>Alert service deployment responsible for tracking and storing alerts when persons of interest are detected. |

## Ingress Configuration and Networking

| Screenshot | Screenshot |
|------------|------------|
| ![Ingress GKE](./screenshots/ingress-gke.png) | ![Running Ingress](./screenshots/running-ingress.png) |
| **Ingress Resource in GKE**<br/>Shows the Ingress configuration in GKE console, routing external traffic to internal services based on hostnames. | **Active Ingress Controller**<br/>Displays the running nginx-ingress-controller with external IP address for accessing the services. |
| ![Installing NGINX Controller](./screenshots/installing-nginx-controller.png) | ![Getting IP Address of Ingress](./screenshots/getting-ip-address-of-ingress.png) |
| **NGINX Ingress Installation**<br/>Installation process of the nginx-ingress-controller via Helm, required for routing external traffic. | **Ingress External IP**<br/>Command output showing how to retrieve the external IP address of the Ingress load balancer for external access. |

## Deployment Process

| Screenshot | Screenshot |
|------------|------------|
| ![Applying Manifests](./screenshots/applying-manifests.png) | ![Building and Pushing Collector Image](./screenshots/building-and-pushing-collector-image.png) |
| **Applying Kubernetes Manifests**<br/>kubectl commands applying the Kubernetes manifests (Deployments, Services, HPA, Ingress) to the GKE cluster. | **Building Collector Docker Image**<br/>Docker build and push process for the custom-built Collector service to Google Container Registry. |

## Testing and API Interaction

| Screenshot | Screenshot |
|------------|------------|
| ![Accessing Collector from Postman](./screenshots/accessing-collector-from-postman.png) | ![Accessing Camera from Postman](./screenshots/accessing-camera-from-postman.png) |
| **Testing Collector API**<br/>Postman request demonstrating successful communication with the Collector service through the Ingress using custom Host header. | **Testing Camera API**<br/>Postman request to the Camera service control interface for configuring and starting camera streams. |
| ![Streaming 100 Requests](./screenshots/streaming-100-requests.png) | ![Snapshot 1](./screenshots/snapshot-1.png) |
| **High-Volume Frame Streaming**<br/>Testing the system under load with 100 frames being streamed from a camera to the Collector service. | **System Snapshot**<br/>General system state snapshot during testing showing service interactions. |
| ![Snapshot 2](./screenshots/snapshot-2.png) | ![Recognized People](./screenshots/recognized-people.png) |
| **Additional System Snapshot**<br/>Another view of the system state during frame processing operations. | **Recognized Persons Output**<br/>Results from the Face Recognition service showing successfully identified persons of interest. |

## Monitoring and Resource Usage

| Screenshot | Screenshot |
|------------|------------|
| ![Install Grafana](./screenshots/install-grafana.png) | ![Collector GCE Dashboard](./screenshots/collector-gce-dashboard.png) |
| **Grafana Installation**<br/>Installation process of Grafana cluster for monitoring the deployed services as recommended in the course forum. | **Collector Dashboard**<br/>Grafana dashboard showing metrics and monitoring data for the Collector service performance. |
| ![Utilization](./screenshots/utilization.png) |  |
| **Resource Utilization**<br/>Overall cluster resource utilization showing CPU and memory usage across all deployed services. | |

## Cost Analysis

| Screenshot | Screenshot |
|------------|------------|
| ![Pricing Estimations](./screenshots/pricing-estimations.png) | ![Pricing Estimations Max](./screenshots/pricing-estimations-max.png) |
| **Average Load Cost Estimation**<br/>GKE pricing calculator showing estimated monthly costs for average load configuration (~$132/month). | **Peak Load Cost Estimation**<br/>Cost estimation for peak load configuration with higher resource allocation (~$152/month). |

