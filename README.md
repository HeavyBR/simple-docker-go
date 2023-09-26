[![progress-banner](https://backend.codecrafters.io/progress/docker/38799e1d-d1ae-4ced-8f67-5c9d331b841c)](https://app.codecrafters.io/users/HeavyBR?r=2qF)

🐳 **Simple Docker Container Runtime in Go**

**Overview**
This project is a basic Docker container runtime implemented in Go for the Codecrafters challenge. It leverages chroot and namespaces for process isolation. While it doesn't use cgroups for resource isolation, it provides a fundamental understanding of containerization concepts.

**Features**
- Process isolation using namespaces.
- Filesystem isolation using chroot.
- A simple command-line interface for managing containers.

