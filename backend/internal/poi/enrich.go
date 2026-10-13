package poi

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/SergeyLazarevv/Klubok/backend/internal/poi/store"
)

// EnrichOptions — параметры пакетной загрузки описаний и фото (команда poi-enrich, ADR-0014).
type EnrichOptions struct {
	Limit      int           // сколько мест за запуск
	MaxAgeDays int           // описания старше — загрузить заново
	Pause      time.Duration // пауза между местами: правила Wikimedia просят не спешить
}

// EnrichStats — итог запуска.
type EnrichStats struct {
	Processed, WithPhoto, WithDescription, Failed int
}

// Enrich загружает описание и фото для мест со ссылками на Википедию, Wikidata или Commons,
// у которых их ещё нет или они старше MaxAgeDays. Ленивая загрузка при открытии карточки остаётся
// для мест, до которых крон ещё не дошёл.
func (s *Service) Enrich(ctx context.Context, opts EnrichOptions) (EnrichStats, error) {
	var st EnrichStats
	rows, err := s.q.ListPOIsToEnrich(ctx, store.ListPOIsToEnrichParams{MaxAgeDays: int32(opts.MaxAgeDays), Lim: int32(opts.Limit)})
	if err != nil {
		return st, fmt.Errorf("list pois to enrich: %w", err)
	}
	for _, r := range rows {
		if err := ctx.Err(); err != nil {
			return st, err
		}
		p := fromRow(store.ListPOIsAroundRow(r))
		lctx, cancel := context.WithTimeout(ctx, detailsTimeout)
		details, qid, err := s.wiki.Lookup(lctx, p)
		cancel()
		if err != nil {
			st.Failed++
			s.log.WarnContext(ctx, "poi enrich", slog.String("osm_id", p.OSMID), slog.Any("err", err))
		} else {
			err = s.q.SavePOIDetails(ctx, store.SavePOIDetailsParams{
				OsmID: p.OSMID, Wikidata: qid, Description: details.Description,
				PhotoUrl: details.PhotoURL, PhotoPage: details.PhotoPage, PhotoCredit: details.PhotoCredit,
			})
			if err != nil {
				return st, fmt.Errorf("save poi details: %w", err)
			}
			st.Processed++
			if details.PhotoURL != "" {
				st.WithPhoto++
			}
			if details.Description != "" {
				st.WithDescription++
			}
		}
		select {
		case <-ctx.Done():
			return st, ctx.Err()
		case <-time.After(opts.Pause):
		}
	}
	return st, nil
}
