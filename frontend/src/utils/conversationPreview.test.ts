import { describe, expect, it } from 'vitest'
import { buildConversationTurnMiddlePreview, buildConversationTurnPreview, normalizeConversationTurnSources } from './conversationPreview'

describe('buildConversationTurnPreview', () => {
  it('limits a single turn to five rendered lines', () => {
    const text = 'aaaaa bbbbb ccccc ddddd eeeee fffff'
    const preview = buildConversationTurnPreview(text, {
      width: 5,
      font: '12px sans-serif',
      maxLines: 5,
      measureText: (value) => value.length,
    })

    const lines = preview.split('\n')
    expect(lines).toHaveLength(5)
    expect(lines[4]).toBe('eeee…')
  })
})

describe('normalizeConversationTurnSources', () => {
  it('does not split fallback text on slash or pipe characters', () => {
    const text = 'Base directory: /private/tmp/project\n| CLI | terminal | type command |'

    expect(normalizeConversationTurnSources(text)).toEqual([text])
  })

  it('uses structured turns when provided', () => {
    expect(normalizeConversationTurnSources('one / two', ['one', 'two'])).toEqual(['one', 'two'])
  })
})

describe('buildConversationTurnMiddlePreview', () => {
  it('keeps the first and last two rendered lines', () => {
    const preview = buildConversationTurnMiddlePreview(
      'one two three four five six seven eight nine ten',
      {
        width: 5,
        font: '12px sans-serif',
        edgeLines: 2,
        measureText: (value) => value.length,
      },
    )

    expect(preview).toEqual({
      head: 'one\ntwo',
      tail: 'nine\nten',
      truncated: true,
    })
  })

  it('does not truncate short text', () => {
    const preview = buildConversationTurnMiddlePreview('one two', {
      width: 5,
      font: '12px sans-serif',
      edgeLines: 2,
      measureText: (value) => value.length,
    })

    expect(preview).toEqual({
      head: 'one two',
      tail: '',
      truncated: false,
    })
  })
})
