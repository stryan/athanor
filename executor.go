package main

import (
	"context"

	"primamateria.systems/materia/pkg/components"
	"primamateria.systems/materia/pkg/containers"
	"primamateria.systems/materia/pkg/services"
)

type Writer struct {
	services.ServiceManager
	containers.ContainerManager
}

func (writer *Writer) InstallComponent(_ *components.Component) error {
	panic("not implemented") // TODO: Implement
}

func (writer *Writer) RemoveComponent(_ *components.Component) error {
	panic("not implemented") // TODO: Implement
}

func (writer *Writer) UpdateComponent(_ *components.Component) error {
	panic("not implemented") // TODO: Implement
}

func (writer *Writer) InstallResource(_ components.Resource, _ []byte) error {
	panic("not implemented") // TODO: Implement
}

func (writer *Writer) RemoveResource(_ components.Resource) error {
	panic("not implemented") // TODO: Implement
}

func (writer *Writer) PurgeComponent(_ *components.Component) error {
	panic("not implemented") // TODO: Implement
}

func (writer *Writer) PurgeComponentByName(_ string) error {
	panic("not implemented") // TODO: Implement
}

func (writer *Writer) Clean() error {
	panic("not implemented") // TODO: Implement
}

func (writer *Writer) InstallScript(_ context.Context, _ string, _ []byte) error {
	panic("not implemented") // TODO: Implement
}

func (writer *Writer) RemoveScript(_ context.Context, _ string) error {
	panic("not implemented") // TODO: Implement
}

func (writer *Writer) InstallUnit(_ context.Context, _ string, _ []byte) error {
	panic("not implemented") // TODO: Implement
}

func (writer *Writer) RemoveUnit(_ context.Context, _ string) error {
	panic("not implemented") // TODO: Implement
}
