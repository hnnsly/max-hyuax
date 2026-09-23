import { Button } from '@maxhub/max-ui';
import { Component, type ReactNode } from 'react';
import { EmptyState, Screen } from '../shared/ui/Layout';

/** Последний рубеж: вместо пустого экрана — понятное сообщение и перезапуск. */
export class ErrorBoundary extends Component<{ children: ReactNode }, { failed: boolean }> {
  state = { failed: false };

  static getDerivedStateFromError() {
    return { failed: true };
  }

  componentDidCatch(error: unknown) {
    console.error('app crashed', error);
  }

  render() {
    if (!this.state.failed) return this.props.children;
    return (
      <Screen title="Мой дом">
        <EmptyState
          title="Что-то пошло не так"
          text="Приложение споткнулось. Перезапустите его, данные не потеряются."
          action={
            <Button variant="primary" size="medium" onClick={() => window.location.reload()}>
              Перезапустить
            </Button>
          }
        />
      </Screen>
    );
  }
}
