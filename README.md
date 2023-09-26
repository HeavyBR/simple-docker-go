🐳 **Simple Docker Container Runtime in Go**

**Overview**
This project is a basic Docker container runtime implemented in Go for the Codecrafters challenge. It leverages chroot and namespaces for process isolation. While it doesn't use cgroups for resource isolation, it provides a fundamental understanding of containerization concepts.

**Features**
- Process isolation using namespaces.
- Filesystem isolation using chroot.
- A simple command-line interface for managing containers.

