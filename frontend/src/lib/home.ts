/** Данные главной: подборка направлений-«открыток». Гербы и фото — из той же базы городов, что и карта. */

const DATA = `${import.meta.env.BASE_URL}data/`

/** Герб-наклейка с белой каймой и тенью (готовит scripts/build-stickers.mjs). */
export const stickerUrl = (id: string) => `${DATA}stickers/${id}.webp`

/** Фото Wikimedia Commons нужной ширины — для карточки шириной ~240 px на ретина-экране. */
export const commonsPhoto = (file: string, width = 480) =>
  `https://commons.wikimedia.org/wiki/Special:FilePath/${encodeURIComponent(file)}?width=${width}`

export interface Postcard {
  id: string // QID города в cities.geojson
  name: string // как ищем в /places
  landmark: string
  photo: string
  abroad?: string
}

/** Подборка — разные по характеру поездки: на выходные, к морю, на север, за границу без визы. */
export const POSTCARDS: Postcard[] = [
  { id: 'Q656', name: 'Санкт-Петербург', landmark: 'Петропавловская крепость', photo: 'Peter & Paul fortress in SPB 03.jpg' },
  { id: 'Q1829', name: 'Калининград', landmark: 'Кафедральный собор', photo: 'Old cathedral of Kaliningrad in Russia.jpg' },
  { id: 'Q131416', name: 'Дербент', landmark: 'Крепость Нарын-кала', photo: 'Derbent. Dagestan.jpg' },
  { id: 'Q2592', name: 'Кострома', landmark: 'Ипатьевский монастырь', photo: 'Ipatiev02.jpg' },
  { id: 'Q1763', name: 'Мурманск', landmark: 'Ледокол «Ленин»', photo: 'Murmansk.jpg' },
  { id: 'Q1953', name: 'Ереван', landmark: 'Гора Арарат', photo: 'Mount Ararat and the Yerevan skyline in spring (28mm).jpg', abroad: 'Армения' },
  { id: 'Q14657', name: 'Выборг', landmark: 'Выборгский замок', photo: 'Vyborg June2012 View from Olaf Tower 06.jpg' },
  { id: 'Q153676', name: 'Кисловодск', landmark: 'Курортный парк', photo: 'Kislovodsk National Park 2, Kislovodsk, Russia.jpg' },
  { id: 'Q2214', name: 'Псков', landmark: 'Псковский кремль', photo: 'Pskov asv07-2018 Kremlin before sunset.jpg' },
  { id: 'Q994', name: 'Тбилиси', landmark: 'Крепость Нарикала', photo: 'View of Tbilisi from Tabori Church 2023-10-08-2.jpg', abroad: 'Грузия' },
  { id: 'Q959', name: 'Владивосток', landmark: 'Золотой мост', photo: 'Views of Vladivostok (October 2024)-0 3.jpg' },
]
