typedef struct {
    char* container_id;
    double cpu_usage;
    double cpu_system;
    double cpu_user;
    double memory_usage;
    double memory_limit;
    double memory_percent;
} ContainerStateResponse;

typedef struct {
    int num_cpu;
    unsigned long long user_cpu;
    unsigned long long idle_cpu;
    unsigned long long system_cpu;
    unsigned long long total_cpu;
    double usage_cpu_percent;
    unsigned long long total_mem;
    unsigned long long used_mem;
    unsigned long long free_mem;
    double free_mem_percent;
    unsigned long long free_storage;
    ContainerStateResponse* containers;
    int containers_count;
} NodeStateResponse;

typedef struct {
    NodeStateResponse* nodes;
    int nodes_count;
} NodesStateResponse;