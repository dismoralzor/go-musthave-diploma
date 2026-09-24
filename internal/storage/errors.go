package storage

import "errors"

// ErrLoginTaken возвращается при попытке зарегистрировать логин, который
// уже занят другим пользователем.
var ErrLoginTaken = errors.New("login already taken")

// ErrUserNotFound возвращается, если пользователь с указанным логином не
// найден.
var ErrUserNotFound = errors.New("user not found")

// ErrOrderOwnedByUser возвращается при повторной загрузке номера заказа тем
// же пользователем, который загрузил его ранее.
var ErrOrderOwnedByUser = errors.New("order already uploaded by this user")

// ErrOrderOwnedByOther возвращается при попытке загрузить номер заказа,
// уже загруженный другим пользователем.
var ErrOrderOwnedByOther = errors.New("order already uploaded by another user")

// ErrInsufficientFunds возвращается при попытке списания, превышающего
// текущий баланс пользователя.
var ErrInsufficientFunds = errors.New("insufficient funds")
