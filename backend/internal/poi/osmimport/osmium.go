package osmimport

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// Osmium — вызовы утилиты osmium-tool (https://osmcode.org/osmium-tool/).
type Osmium struct {
	bin string
}

func NewOsmium(bin string) *Osmium {
	return &Osmium{bin: bin}
}

// Check — установлен ли osmium.
func (o *Osmium) Check(ctx context.Context) error {
	if _, err := o.run(ctx, "--version"); err != nil {
		return fmt.Errorf("osmium не найден (%s): установите osmium-tool — sudo apt install osmium-tool: %w", o.bin, err)
	}
	return nil
}

// Sequence — номер дифа Geofabrik, на котором собрана выгрузка (из заголовка файла).
func (o *Osmium) Sequence(ctx context.Context, pbf string) (int, error) {
	out, err := o.run(ctx, "fileinfo", "-e", "-g", "header.option.osmosis_replication_sequence_number", pbf)
	if err != nil {
		return 0, err
	}
	seq, err := strconv.Atoi(strings.TrimSpace(out))
	if err != nil {
		return 0, fmt.Errorf("osmium: в %s нет номера дифа в заголовке", pbf)
	}
	return seq, nil
}

// ApplyChanges накладывает дифы на выгрузку и записывает в заголовок новый номер дифа.
func (o *Osmium) ApplyChanges(ctx context.Context, base string, diffs []string, out string, seq int) error {
	args := append([]string{"apply-changes", base}, diffs...)
	args = append(args, "-o", out, "--overwrite",
		"--output-header=osmosis_replication_sequence_number="+strconv.Itoa(seq))
	_, err := o.run(ctx, args...)
	return err
}

// tagFilters — какие объекты нужны слоям карты (ADR-0014). Лишнее отсеивает poi.Classify.
var tagFilters = []string{
	"nwr/amenity=fuel,charging_station,cafe,restaurant,fast_food,toilets,parking,atm,bureau_de_change,pharmacy,bus_station",
	"nwr/highway=rest_area,services",
	"nwr/barrier=border_control",
	"nwr/shop=car_repair,tyres",
	"nwr/railway=station",
	"nwr/tourism",
	"nwr/historic",
	"nwr/natural=beach,waterfall,peak,cave_entrance,spring",
	"nwr/leisure=park,nature_reserve,beach_resort",
	// Любое туристическое место с фото или статьёй
	"nwr/wikidata", "nwr/wikipedia", "nwr/wikimedia_commons", "nwr/image",
}

// TagsFilter оставляет в выгрузке только нужные объекты.
func (o *Osmium) TagsFilter(ctx context.Context, in, out string) error {
	args := append([]string{"tags-filter", in}, tagFilters...)
	args = append(args, "-o", out, "--overwrite")
	_, err := o.run(ctx, args...)
	return err
}

// Export выгружает объекты в GeoJSON Lines: одна строка — один объект; id вида n123, w456, a789.
func (o *Osmium) Export(ctx context.Context, in, out string) error {
	_, err := o.run(ctx, "export", in, "-f", "geojsonseq", "-o", out, "--overwrite",
		"--add-unique-id=type_id", "--geometry-types=point,linestring,polygon",
		"--format-option", "print_record_separator=false")
	return err
}

func (o *Osmium) run(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, o.bin, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("osmium %s: %w: %s", args[0], err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}
