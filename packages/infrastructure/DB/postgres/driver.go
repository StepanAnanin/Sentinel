package postgres

// TODO add logs

type postgers struct {
    connector
    seeker
    repository
}

var driver *postgers

func InitDriver() *postgers {
    driver = new(postgers)

    return driver
}

