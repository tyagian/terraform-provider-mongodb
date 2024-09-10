package mongodb

type User struct {
	Username string `bson:"user"`
	Password string

	Database   string     `bson:"db"`
	Roles      ShortRoles `bson:"roles"`
	Mechanisms []string   `bson:"mechanisms"`
}

type Result struct {
	Ok int `bson:"ok"`
}
