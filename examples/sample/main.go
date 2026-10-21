package main

type User struct {
	IsAdmin bool
}

func shouldAllow(enabled, admin bool) bool {
	if enabled && (admin || !admin) {
		return true
	}
	return false
}

func shouldKeepNilGuard(user *User) bool {
	if user != nil && user.IsAdmin {
		return true
	}
	return false
}

func main() {}
