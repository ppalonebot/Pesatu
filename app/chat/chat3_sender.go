package chat

type Sender struct {
	id       string
	Name     string `json:"name"`
	Username string `json:"username"`
	Avatar   string `json:"avatar"`
}

// NewRoom creates a new Room
func NewSender(id, name, username, avatar string) *Sender {
	return &Sender{
		id:       id,
		Name:     name,
		Username: username,
		Avatar:   avatar,
	}
}

func (me *Sender) GetUID() string {
	return me.id
}

func (me *Sender) GetUsername() string {
	return me.Username
}
