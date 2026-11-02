package transit

import "time"

func (d *Data) UseHiddenLines() error {
	names, err := d.store.ReadHiddenLines()
	if err != nil {
		return err
	}
	d.hiddenLines.store(names)
	return nil
}

func (d *Data) SetLinesHidden(names []string, hidden bool) error {
	if err := d.store.SaveHiddenLines(names, hidden, time.Now()); err != nil {
		return err
	}
	return d.UseHiddenLines()
}

func (d *Data) HiddenLines() []string { return d.hiddenLines.sorted() }

func (d *Data) LineHidden(routeShortName string) bool { return d.hiddenLines.has(routeShortName) }
