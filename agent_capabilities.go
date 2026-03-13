package tools

type TaskSubmitter interface {
	SubmitTask(input string) error
}

type ConfigPathProvider interface {
	GetConfigPath() string
}
