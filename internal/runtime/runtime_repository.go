package runtime

type RuntimeRepository interface {
	CreateRuntime(key string, runtime Runtime) error
	GetRuntime(key string, runtime RuntimeType) (Runtime, error)
	GetAllRuntimes(runtime RuntimeType) ([]Runtime, error)
	UpdateRuntime(key string, runtime Runtime) error
}
