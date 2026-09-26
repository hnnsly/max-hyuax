import { CellList, CellSimple, Switch } from '@maxhub/max-ui';
import { createContext, useContext, useEffect, useId, useState } from 'react';
import { deviceStore, loadA11y, saveA11y, type A11yMode } from '../shared/lib/a11y';
import { Section } from '../shared/ui/Layout';

type A11yValue = [A11yMode, (mode: A11yMode) => void];

export const A11yContext = createContext<A11yValue>(['normal', () => {}]);

/**
 * Состояние режима «Крупно и контрастно» для корня приложения. Атрибут data-a11y ставится и на
 * <html>: MAX UI задаёт размеры шрифтов в rem, и крупный режим увеличивает корневой размер.
 */
export function useA11yState(): A11yValue {
  const [mode, setMode] = useState<A11yMode>(() => loadA11y(window.location.search, deviceStore()));
  useEffect(() => {
    document.documentElement.dataset.a11y = mode;
    saveA11y(mode, deviceStore());
  }, [mode]);
  return [mode, setMode];
}

/**
 * Ячейки MAX UI с onClick — это div с role="button": экранный чтец на телефоне их нажимает,
 * а клавиатура и пульты нет. Enter и пробел делают click, как у настоящей кнопки (WCAG 2.1.1).
 */
export function useRoleButtonKeys() {
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key !== 'Enter' && e.key !== ' ') return;
      const t = e.target;
      if (!(t instanceof HTMLElement) || t instanceof HTMLButtonElement || t.getAttribute('role') !== 'button') return;
      e.preventDefault();
      t.click();
    };
    document.addEventListener('keydown', onKey);
    return () => document.removeEventListener('keydown', onKey);
  }, []);
}

/** Переключатель режима в блоке «Удобство» внизу главных экранов. */
export function A11yToggle() {
  const [mode, setMode] = useContext(A11yContext);
  const id = useId();
  return (
    <>
      <Section title="Удобство" />
      <CellList mode="island">
        <CellSimple
          title={<label htmlFor={id}>Крупно и контрастно</label>}
          subtitle="Крупный текст, контрастные подписи и кнопки побольше"
          after={<Switch id={id} checked={mode === 'large'} onChange={(e) => setMode(e.currentTarget.checked ? 'large' : 'normal')} />}
        />
      </CellList>
    </>
  );
}
