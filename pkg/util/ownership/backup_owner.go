package ownership

type Ownership struct {
	Username   string
	DomainName string
}

func GetBackupOwner() Ownership {
	return Ownership{
		Username:   "default",
		DomainName: "default",
	}
}
