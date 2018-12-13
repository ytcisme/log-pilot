package container

type Container struct {
	ID        string
	Name      string
	Namespace string
	// Pod Name
	Pod   string
	PodID string
	// Container Name
}
