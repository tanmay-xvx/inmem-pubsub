package registry

import "errors"

var (
	// ErrInvalidTopicName is returned when an invalid topic name is provided
	ErrInvalidTopicName = errors.New("invalid topic name")

	// ErrTopicAlreadyExists is returned when trying to create a topic that already exists
	ErrTopicAlreadyExists = errors.New("topic already exists")

	// ErrTopicNotFound is returned when trying to access a topic that doesn't exist
	ErrTopicNotFound = errors.New("topic not found")
)
