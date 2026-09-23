import { describe, expect, it } from 'vitest';
import { stackReducer, type Route } from './stack';

const home: Route = { name: 'home' };
const issue: Route = { name: 'issue', id: '1' };

describe('стек экранов', () => {
  it('push и back', () => {
    const s = stackReducer([home], { type: 'push', route: issue });
    expect(s).toEqual([home, issue]);
    expect(stackReducer(s, { type: 'back' })).toEqual([home]);
  });

  it('back на корне ничего не делает', () => {
    expect(stackReducer([home], { type: 'back' })).toEqual([home]);
  });

  it('reset заменяет стек целиком, пустой — не допускается', () => {
    expect(stackReducer([home, issue], { type: 'reset', routes: [{ name: 'uk' }] })).toEqual([{ name: 'uk' }]);
    expect(stackReducer([home], { type: 'reset', routes: [] })).toEqual([home]);
  });
});
