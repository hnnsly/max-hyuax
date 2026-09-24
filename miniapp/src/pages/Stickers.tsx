import { Button } from '@maxhub/max-ui';
import { Printer } from '@phosphor-icons/react';
import { useState } from 'react';
import { api } from '../shared/api/client';
import type { AssetObject, HouseDetails } from '../shared/api/types';
import { useResource } from '../shared/api/useResource';
import { appLink, BOT_NAME } from '../shared/bridge/bridge';
import { capitalize, splitAddress } from '../shared/lib/format';
import { objectStartParam, qrPath, stickerCopy } from '../shared/lib/sticker';
import { EmptyState, ErrorState, Island, Loading } from '../shared/ui/Layout';
import s from './stickers.module.css';

/** Наклейки с QR-кодами для объектов домов УК: выбор дома и объекта, превью, печать A6. */
export function StickersView() {
  const houses = useResource(() => api.ukHouses(), []);
  const [houseId, setHouseId] = useState('');
  const current = houseId || houses.data?.[0]?.id || '';

  if (houses.loading && !houses.data) return <Loading />;
  if (houses.error || !houses.data) {
    return <ErrorState title="Не удалось загрузить дома" message={houses.error?.message ?? ''} onRetry={houses.reload} />;
  }
  if (houses.data.length === 0) {
    return <EmptyState title="Домов пока нет" text="Когда за УК закрепят дома, здесь появятся наклейки для их объектов." />;
  }
  return (
    <>
      <div className={s.chips} role="group" aria-label="Дом">
        {houses.data.map((h) => (
          <button key={h.id} type="button" className={s.chip} aria-pressed={h.id === current} onClick={() => setHouseId(h.id)}>
            {splitAddress(h.address).number || h.address}
          </button>
        ))}
      </div>
      <HouseStickers key={current} houseId={current} />
    </>
  );
}

function HouseStickers({ houseId }: { houseId: string }) {
  const res = useResource(() => api.house(houseId), [houseId]);
  const [objectId, setObjectId] = useState('');

  if (res.loading && !res.data) return <Loading />;
  if (res.error || !res.data) {
    return <ErrorState title="Не удалось загрузить объекты дома" message={res.error?.message ?? ''} onRetry={res.reload} />;
  }
  const house = res.data;
  if (house.objects.length === 0) {
    return <EmptyState title="Нет объектов с QR-кодами" text="Объекты дома с QR-кодами добавляет администратор сервиса." />;
  }
  const object = house.objects.find((o) => o.id === objectId) ?? house.objects[0]!;
  return (
    <>
      <div className={s.chips} role="group" aria-label="Объект">
        {house.objects.map((o) => {
          const entrance = house.entrances.find((e) => e.id === o.entrance_id);
          const plate = stickerCopy(o.category, capitalize(o.label), Boolean(entrance)).plate;
          return (
            <button key={o.id} type="button" className={s.chip} aria-pressed={o.id === object.id} onClick={() => setObjectId(o.id)}>
              {entrance ? `${plate}, подъезд ${entrance.number}` : plate}
            </button>
          );
        })}
      </div>
      <Island style={{ overflow: 'hidden' }}>
        <Sticker house={house} object={object} />
      </Island>
      <Button variant="primary" size="large" stretched iconBefore={<Printer size={20} />} onClick={() => window.print()}>
        Распечатать
      </Button>
      <p className={s.hint}>Формат A6, для кабины лифта или двери подъезда. Печатайте из браузера на компьютере.</p>
    </>
  );
}

/** Наклейка по холсту Sticker: всегда светлая, потому что печатается на бумаге. */
export function Sticker({ house, object }: { house: HouseDetails; object: AssetObject }) {
  const entrance = house.entrances.find((e) => e.id === object.entrance_id);
  const copy = stickerCopy(object.category, capitalize(object.label.split(', ').at(-1) ?? object.label), Boolean(entrance));
  const param = objectStartParam(object.qr_code);
  const qr = param ? qrPath(appLink(param)) : null;
  const phone = house.organization.phone_dispatcher;
  return (
    <article className={s.sticker} aria-label={`Наклейка: ${copy.plate}, ${house.address}`}>
      <div className={s.plate}>
        <div className={s.plateText}>
          <span className={s.plateAddress}>{house.address}</span>
          <span className={s.plateWord}>{copy.plate}</span>
        </div>
        {entrance && <span className={s.plateBadge}>подъезд {entrance.number}</span>}
      </div>

      <div className={s.lead}>
        <h2 className={s.question}>{copy.question}</h2>
        <p className={s.sub}>Сообщите соседям и в управляющую компанию за минуту.</p>
      </div>

      <div className={s.body}>
        <div className={s.qr}>
          {qr ? (
            <svg viewBox={`0 0 ${qr.size} ${qr.size}`} shapeRendering="crispEdges" role="img" aria-label="QR-код заявки по этому объекту">
              <rect width={qr.size} height={qr.size} fill="#fff" />
              <path d={qr.d} fill="#0a0b0d" />
            </svg>
          ) : (
            <span className={s.qrError}>Код объекта не подходит для QR</span>
          )}
        </div>
        <ol className={s.steps}>
          <li>Наведите камеру телефона на код</li>
          <li>{copy.opens}</li>
          <li>Соседи присоединятся, УК увидит, сколько человек ждёт ремонта</li>
        </ol>
      </div>

      {phone && (
        <div className={s.emergency}>
          <div className={s.emergencyText}>
            <span className={s.emergencyTitle}>{copy.emergency}</span>
            <span>Звоните в диспетчерскую, не пишите заявку</span>
          </div>
          <span className={s.phone}>{phone}</span>
        </div>
      )}

      <div className={s.foot}>
        <span>max.ru/{BOT_NAME}</span>
        <span>наклейка A6</span>
      </div>
    </article>
  );
}
