package plugin

type Plugin interface {
	Init() error

	Execute(input interface{}) (interface{}, error)

	Close() error
}

func Auto() {

}
