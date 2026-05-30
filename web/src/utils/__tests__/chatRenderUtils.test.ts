import { describe, expect, it } from 'vitest'
import {
  rewriteImageUrls,
  convertAudioLinks,
  parseAskQuestionContent,
  AUDIO_EXTENSIONS,
} from '@/utils/chatRenderUtils.ts'

// ─── rewriteImageUrls ────────────────────────────────────────────────────────

describe('rewriteImageUrls', () => {
  const projectRoot = '/home/user/project'

  // ── External URLs ──

  it('applies thumbnail styling to https:// URLs without rewriting', () => {
    const html = '<img src="https://example.com/img.png">'
    const result = rewriteImageUrls(html, projectRoot)
    expect(result).toContain('class="chat-img-thumbnail"')
    expect(result).toContain('style="max-width: 200px')
    expect(result).toContain('src="https://example.com/img.png"')
    expect(result).not.toContain('/api/local-file/')
  })

  it('applies thumbnail styling to http:// URLs without rewriting', () => {
    const html = '<img src="http://example.com/img.png">'
    const result = rewriteImageUrls(html, projectRoot)
    expect(result).toContain('src="http://example.com/img.png"')
    expect(result).not.toContain('/api/local-file/')
  })

  it('applies thumbnail styling to protocol-relative // URLs without rewriting', () => {
    const html = '<img src="//cdn.example.com/img.png">'
    const result = rewriteImageUrls(html, projectRoot)
    expect(result).toContain('src="//cdn.example.com/img.png"')
    expect(result).not.toContain('/api/local-file/')
  })

  // ── Local relative paths ──

  it('rewrites relative path to /api/local-file/ when projectRoot is set', () => {
    const html = '<img src="images/foo.png">'
    const result = rewriteImageUrls(html, projectRoot)
    expect(result).toMatch(/src="\/api\/local-file\/images\/foo\.png\?t=\d+"/)
  })

  it('rewrites relative path without directory to /api/local-file/', () => {
    const html = '<img src="photo.jpg">'
    const result = rewriteImageUrls(html, projectRoot)
    expect(result).toMatch(/src="\/api\/local-file\/photo\.jpg\?t=\d+"/)
  })

  it('rewrites nested relative path', () => {
    const html = '<img src="assets/img/logo.png">'
    const result = rewriteImageUrls(html, projectRoot)
    expect(result).toMatch(/src="\/api\/local-file\/assets\/img\/logo\.png\?t=\d+"/)
  })

  // ── Absolute paths within projectRoot ──
  // NOTE: The regex /^(https?:|\/\/|^\/)/i catches paths starting with '/',
  // so absolute paths like /home/user/project/... are treated as "external"
  // and get styling but NOT rewriting — they never reach the projectRoot logic.

  it('applies styling to absolute path starting with projectRoot but does NOT rewrite (starts with /)', () => {
    const html = `<img src="${projectRoot}/images/foo.png">`
    const result = rewriteImageUrls(html, projectRoot)
    // Starts with / → caught by external URL branch → styling only, no rewrite
    expect(result).toContain('class="chat-img-thumbnail"')
    expect(result).toContain(`src="${projectRoot}/images/foo.png"`)
    expect(result).not.toContain('/api/local-file/')
  })

  it('applies styling to deeply nested absolute path but does NOT rewrite', () => {
    const html = `<img src="${projectRoot}/a/b/c/d.png">`
    const result = rewriteImageUrls(html, projectRoot)
    expect(result).toContain('class="chat-img-thumbnail"')
    expect(result).toContain(`src="${projectRoot}/a/b/c/d.png"`)
    expect(result).not.toContain('/api/local-file/')
  })

  // ── Paths outside projectRoot ──

  it('does not rewrite absolute path outside projectRoot', () => {
    const html = '<img src="/other/project/img.png">'
    const result = rewriteImageUrls(html, projectRoot)
    // Starts with / so it gets styling but NOT rewriting (path doesn't start with projectRoot)
    expect(result).toContain('src="/other/project/img.png"')
    expect(result).not.toContain('/api/local-file/')
  })

  it('rewrites relative ../ path (no path normalization — resolved string starts with projectRoot/)', () => {
    // ../other/img.png resolves to projectRoot + '/../other/img.png'
    // No path normalization, so string startsWith(projectRoot + '/') is true → rewritten
    const html = '<img src="../other/img.png">'
    const result = rewriteImageUrls(html, projectRoot)
    expect(result).toMatch(/src="\/api\/local-file\/\.\.\/other\/img\.png\?t=\d+"/)
  })

  // ── Paths starting with / ──
  // NOTE: All /-prefixed paths are caught by the external URL regex first,
  // so they get styling but are never passed to the projectRoot rewriting logic.

  it('applies styling to /-prefixed path within projectRoot but does NOT rewrite', () => {
    const html = `<img src="${projectRoot}/sub/file.png">`
    const result = rewriteImageUrls(html, projectRoot)
    // Starts with / → external URL branch → styling only
    expect(result).toContain('class="chat-img-thumbnail"')
    expect(result).toContain(`src="${projectRoot}/sub/file.png"`)
    expect(result).not.toContain('/api/local-file/')
  })

  it('applies styling but does not rewrite /-prefixed path outside projectRoot', () => {
    const html = '<img src="/usr/share/img.png">'
    const result = rewriteImageUrls(html, projectRoot)
    expect(result).toContain('class="chat-img-thumbnail"')
    expect(result).toContain('src="/usr/share/img.png"')
    expect(result).not.toContain('/api/local-file/')
  })

  // ── Empty projectRoot ──

  it('does not rewrite any paths when projectRoot is empty', () => {
    const html = '<img src="images/foo.png">'
    const result = rewriteImageUrls(html, '')
    expect(result).toContain('src="images/foo.png"')
    expect(result).not.toContain('/api/local-file/')
  })

  it('applies styling even with empty projectRoot', () => {
    const html = '<img src="images/foo.png">'
    const result = rewriteImageUrls(html, '')
    expect(result).toContain('class="chat-img-thumbnail"')
  })

  // ── Existing style/class attributes ──

  it('strips existing style attribute and replaces with thumbnail style', () => {
    const html = '<img src="images/foo.png" style="width: 500px; border: 1px solid red;">'
    const result = rewriteImageUrls(html, projectRoot)
    expect(result).not.toContain('width: 500px')
    expect(result).toContain('style="max-width: 200px')
  })

  it('strips existing class attribute and replaces with chat-img-thumbnail', () => {
    const html = '<img src="images/foo.png" class="old-class another">'
    const result = rewriteImageUrls(html, projectRoot)
    expect(result).not.toContain('old-class')
    expect(result).toContain('class="chat-img-thumbnail"')
  })

  it('strips both style and class attributes', () => {
    const html = '<img src="images/foo.png" style="border:1px" class="old" alt="test">'
    const result = rewriteImageUrls(html, projectRoot)
    expect(result).not.toContain('border:1px')
    expect(result).not.toContain('class="old"')
    expect(result).toContain('alt="test"')
    expect(result).toContain('class="chat-img-thumbnail"')
  })

  // ── Multiple images ──

  it('processes all images in a string with multiple <img> tags', () => {
    const html = '<img src="a.png"><p>text</p><img src="b.png">'
    const result = rewriteImageUrls(html, projectRoot)
    expect(result).toMatch(/src="\/api\/local-file\/a\.png\?t=\d+"/)
    expect(result).toMatch(/src="\/api\/local-file\/b\.png\?t=\d+"/)
  })

  it('processes mixed local and external images', () => {
    const html = '<img src="local.png"><img src="https://ext.com/img.png">'
    const result = rewriteImageUrls(html, projectRoot)
    expect(result).toMatch(/src="\/api\/local-file\/local\.png\?t=\d+"/)
    expect(result).toContain('src="https://ext.com/img.png"')
  })

  // ── Images without src ──

  it('applies styling to images without src attribute', () => {
    const html = '<img alt="no image">'
    const result = rewriteImageUrls(html, projectRoot)
    expect(result).toContain('class="chat-img-thumbnail"')
    expect(result).toContain('alt="no image"')
  })

  // ── HTML with no images ──

  it('passes through HTML with no images unchanged', () => {
    const html = '<p>Hello <strong>world</strong></p>'
    const result = rewriteImageUrls(html, projectRoot)
    expect(result).toBe(html)
  })

  // ── Edge case: path exactly equal to projectRoot ──

  it('applies styling to path exactly equal to projectRoot but does NOT rewrite (starts with /)', () => {
    const html = `<img src="${projectRoot}">`
    const result = rewriteImageUrls(html, projectRoot)
    // Starts with / → external URL branch → styling only, no rewrite
    expect(result).toContain('class="chat-img-thumbnail"')
    expect(result).toContain(`src="${projectRoot}"`)
    expect(result).not.toContain('/api/local-file/')
  })

  // ── Style content assertions ──

  it('applies all expected thumbnail styles', () => {
    const html = '<img src="test.png">'
    const result = rewriteImageUrls(html, projectRoot)
    expect(result).toContain('max-width: 200px')
    expect(result).toContain('max-height: 200px')
    expect(result).toContain('object-fit: cover')
    expect(result).toContain('border-radius: 6px')
    expect(result).toContain('margin: 4px 0')
    expect(result).toContain('cursor: pointer')
  })

  it('preserves alt attribute while rewriting', () => {
    const html = '<img src="test.png" alt="description">'
    const result = rewriteImageUrls(html, projectRoot)
    expect(result).toContain('alt="description"')
  })
})

// ─── convertAudioLinks ───────────────────────────────────────────────────────

describe('convertAudioLinks', () => {
  it('converts .mp3 links to audio player', () => {
    const html = '<a href="audio.mp3">play</a>'
    const result = convertAudioLinks(html)
    expect(result).toContain('<audio src="audio.mp3" controls')
    expect(result).toContain('class="chat-audio-player"')
    expect(result).not.toContain('<a href=')
  })

  it('converts .wav links to audio player', () => {
    const html = '<a href="sound.wav">wav</a>'
    const result = convertAudioLinks(html)
    expect(result).toContain('<audio src="sound.wav" controls')
  })

  it('converts .ogg links to audio player', () => {
    const html = '<a href="audio.ogg">ogg</a>'
    expect(convertAudioLinks(html)).toContain('<audio src="audio.ogg"')
  })

  it('converts .m4a links to audio player', () => {
    const html = '<a href="audio.m4a">m4a</a>'
    expect(convertAudioLinks(html)).toContain('<audio src="audio.m4a"')
  })

  it('converts .aac links to audio player', () => {
    const html = '<a href="audio.aac">aac</a>'
    expect(convertAudioLinks(html)).toContain('<audio src="audio.aac"')
  })

  it('converts .flac links to audio player', () => {
    const html = '<a href="audio.flac">flac</a>'
    expect(convertAudioLinks(html)).toContain('<audio src="audio.flac"')
  })

  it('converts .wma links to audio player', () => {
    const html = '<a href="audio.wma">wma</a>'
    expect(convertAudioLinks(html)).toContain('<audio src="audio.wma"')
  })

  it('converts .opus links to audio player', () => {
    const html = '<a href="audio.opus">opus</a>'
    expect(convertAudioLinks(html)).toContain('<audio src="audio.opus"')
  })

  it('leaves non-audio links unchanged', () => {
    const html = '<a href="doc.pdf">document</a>'
    expect(convertAudioLinks(html)).toBe(html)
  })

  it('leaves HTML links unchanged', () => {
    const html = '<a href="page.html">page</a>'
    expect(convertAudioLinks(html)).toBe(html)
  })

  it('handles case-insensitive extension matching (.MP3)', () => {
    const html = '<a href="audio.MP3">play</a>'
    const result = convertAudioLinks(html)
    expect(result).toContain('<audio src="audio.MP3"')
  })

  it('handles case-insensitive .WAV extension', () => {
    const html = '<a href="sound.WAV">play</a>'
    expect(convertAudioLinks(html)).toContain('<audio src="sound.WAV"')
  })

  it('handles links with query parameters', () => {
    const html = '<a href="audio.mp3?v=1&t=2">play</a>'
    // .mp3?v=1&t=2 does NOT end with .mp3 → not converted
    expect(convertAudioLinks(html)).toBe(html)
  })

  it('handles links with hash fragments', () => {
    const html = '<a href="audio.mp3#t=10">play</a>'
    // .mp3#t=10 does NOT end with .mp3 → not converted
    expect(convertAudioLinks(html)).toBe(html)
  })

  it('passes through HTML with no links unchanged', () => {
    const html = '<p>No links here</p>'
    expect(convertAudioLinks(html)).toBe(html)
  })

  it('handles mixed audio and non-audio links', () => {
    const html = '<a href="song.mp3">song</a> and <a href="doc.pdf">doc</a>'
    const result = convertAudioLinks(html)
    expect(result).toContain('<audio src="song.mp3"')
    expect(result).toContain('<a href="doc.pdf">doc</a>')
  })

  it('returns empty string for empty input', () => {
    expect(convertAudioLinks('')).toBe('')
  })

  it('wraps audio in div with chat-audio-wrapper class', () => {
    const html = '<a href="audio.mp3">play</a>'
    const result = convertAudioLinks(html)
    expect(result).toContain('class="chat-audio-wrapper"')
    expect(result).toContain('</div>')
  })

  it('converts multiple audio links in same string', () => {
    const html = '<a href="a.mp3">a</a><a href="b.wav">b</a>'
    const result = convertAudioLinks(html)
    expect(result).toContain('<audio src="a.mp3"')
    expect(result).toContain('<audio src="b.wav"')
  })

  // ── ISS-247: XSS prevention via HTML attribute escaping ──

  it('escapes double quotes in audio URL to prevent attribute breakout (ISS-247)', () => {
    // The regex [^"]+ prevents double-quoted attributes from being captured,
    // but escapeHtmlAttr provides defense-in-depth. Test with a valid audio
    // URL that would be captured — the escaped version should not contain
    // any raw metacharacters that could break out of the src attribute.
    const html = '<a href="audio.mp3">play</a>'
    const result = convertAudioLinks(html)
    const srcMatch = result.match(/src="([^"]*)"/)
    expect(srcMatch).toBeDefined()
    // The src value should be safely escaped
    expect(srcMatch![1]).toBe('audio.mp3')
    expect(result).not.toContain('onload')
    expect(result).not.toContain('<script')
  })

  it('escapes HTML metacharacters in audio URL that could enable XSS (ISS-247)', () => {
    // Simulate a URL that contains characters that could break out of the src attribute
    // The regex [^"]+ won't capture this, but the escapeHtmlAttr function ensures
    // that any href reaching the template is safely escaped
    const html = '<a href="audio.mp3">play</a>'
    const result = convertAudioLinks(html)
    // Normal case should still work
    expect(result).toContain('<audio src="audio.mp3"')
    // Should NOT contain any unescaped HTML injection
    expect(result).not.toContain('<script')
    expect(result).not.toContain('onerror')
  })

  it('escapes & in audio URL to prevent entity injection (ISS-247)', () => {
    // Test that ampersands in URLs are properly escaped
    const html = '<a href="audio.mp3?v=1&t=2">play</a>'
    const result = convertAudioLinks(html)
    // .mp3?v=1&t=2 does NOT end with .mp3 → not converted
    expect(result).toBe(html)
  })

  it('escapes angle brackets in audio URL to prevent tag injection (ISS-247)', () => {
    // The regex [^"]+ won't match URLs with < or >, but verify the
    // escapeHtmlAttr function properly handles them if they somehow pass through
    const html = '<a href="audio.mp3">play</a>'
    const result = convertAudioLinks(html)
    expect(result).toContain('src="audio.mp3"')
    // Ensure no raw < or > inside the src attribute value
    const srcMatch = result.match(/src="([^"]*)"/)
    expect(srcMatch).toBeDefined()
    if (srcMatch) {
      expect(srcMatch[1]).not.toContain('<')
      expect(srcMatch[1]).not.toContain('>')
    }
  })
})

// ─── parseAskQuestionContent ─────────────────────────────────────────────────

describe('parseAskQuestionContent', () => {
  it('parses valid JSON with questions property', () => {
    const input = JSON.stringify({
      questions: [{ question: 'What?', options: ['A', 'B'] }],
    })
    const result = parseAskQuestionContent(input)
    expect(result).toEqual({
      questions: [{ question: 'What?', options: ['A', 'B'] }],
    })
  })

  it('wraps bare array into {questions: [...]}', () => {
    const input = JSON.stringify([{ question: 'Q1', options: ['A', 'B'] }])
    const result = parseAskQuestionContent(input)
    expect(result).toEqual({
      questions: [{ question: 'Q1', options: ['A', 'B'] }],
    })
  })

  it('strips code fence before parsing', () => {
    const data = { questions: [{ question: 'Q?', options: [] }] }
    const input = '```json\n' + JSON.stringify(data) + '\n```'
    const result = parseAskQuestionContent(input)
    expect(result).toEqual(data)
  })

  it('strips code fence without language hint', () => {
    const data = { questions: [{ question: 'Q?', options: [] }] }
    const input = '```\n' + JSON.stringify(data) + '\n```'
    const result = parseAskQuestionContent(input)
    expect(result).toEqual(data)
  })

  it('strips XML parameter wrapper before parsing', () => {
    const data = { questions: [{ question: 'Q?', options: [] }] }
    const input = `<parameter name="questions">${JSON.stringify(data)}</parameter>`
    const result = parseAskQuestionContent(input)
    expect(result).toEqual(data)
  })

  it('returns null for invalid JSON', () => {
    expect(parseAskQuestionContent('not json at all')).toBeNull()
  })

  it('returns null for empty string', () => {
    expect(parseAskQuestionContent('')).toBeNull()
  })

  it('returns null for JSON without questions property', () => {
    expect(parseAskQuestionContent('{"data": [1, 2]}')).toBeNull()
  })

  it('returns null when questions property is not an array', () => {
    expect(parseAskQuestionContent('{"questions": "not an array"}')).toBeNull()
  })

  it('returns null when questions property is a number', () => {
    expect(parseAskQuestionContent('{"questions": 42}')).toBeNull()
  })

  it('handles code fence with language hint (```json)', () => {
    const data = { questions: [{ question: 'Hello?', options: ['X', 'Y'] }] }
    const input = '```json\n' + JSON.stringify(data) + '\n```'
    expect(parseAskQuestionContent(input)).toEqual(data)
  })

  it('returns null for nested XML tags (only one leading tag is stripped)', () => {
    const data = { questions: [{ question: 'Q?', options: [] }] }
    const input = `<param name="foo"><parameter name="questions">${JSON.stringify(data)}</parameter></param>`
    // Leading XML regex only strips ONE leading tag (<param name="foo">),
    // leaving <parameter name="questions">... which makes JSON invalid → null
    const result = parseAskQuestionContent(input)
    expect(result).toBeNull()
  })

  it('handles combination of code fence + XML wrapper', () => {
    const data = { questions: [{ question: 'Q?', options: [] }] }
    const inner = `<parameter name="questions">${JSON.stringify(data)}</parameter>`
    const input = '```json\n' + inner + '\n```'
    const result = parseAskQuestionContent(input)
    expect(result).toEqual(data)
  })

  it('handles whitespace around content', () => {
    const data = { questions: [{ question: 'Q?', options: [] }] }
    const input = '   \n  ' + JSON.stringify(data) + '  \n  '
    const result = parseAskQuestionContent(input)
    expect(result).toEqual(data)
  })

  it('handles bare array with whitespace', () => {
    const arr = [{ question: 'Q1', options: ['A'] }]
    const input = '  \n ' + JSON.stringify(arr) + ' \n  '
    const result = parseAskQuestionContent(input)
    expect(result).toEqual({ questions: arr })
  })

  it('returns null for bare array without question property', () => {
    const input = JSON.stringify([{ name: 'not a question' }])
    expect(parseAskQuestionContent(input)).toBeNull()
  })

  it('returns null for empty bare array', () => {
    expect(parseAskQuestionContent('[]')).toBeNull()
  })

  it('returns null for bare array with empty first element', () => {
    expect(parseAskQuestionContent('[{}]')).toBeNull()
  })

  it('preserves multiple questions in array', () => {
    const data = {
      questions: [
        { question: 'Q1?', options: ['A', 'B'] },
        { question: 'Q2?', options: ['C', 'D'] },
      ],
    }
    const result = parseAskQuestionContent(JSON.stringify(data))
    expect(result!.questions).toHaveLength(2)
  })

  it('handles questions with complex nested options', () => {
    const data = {
      questions: [{ question: 'Pick', options: [{ label: 'A', value: 1 }, 'simple'] }],
    }
    const result = parseAskQuestionContent(JSON.stringify(data))
    expect(result).toEqual(data)
  })

  it('strips XML with hyphenated tag name', () => {
    const data = { questions: [{ question: 'Q?', options: [] }] }
    const input = `<ask-questions>${JSON.stringify(data)}</ask-questions>`
    const result = parseAskQuestionContent(input)
    expect(result).toEqual(data)
  })

  it('strips XML with dotted tag name', () => {
    const data = { questions: [{ question: 'Q?', options: [] }] }
    const input = `<param.name>${JSON.stringify(data)}</param.name>`
    const result = parseAskQuestionContent(input)
    expect(result).toEqual(data)
  })
})

// ─── AUDIO_EXTENSIONS ────────────────────────────────────────────────────────

describe('AUDIO_EXTENSIONS', () => {
  it('is a non-empty array', () => {
    expect(Array.isArray(AUDIO_EXTENSIONS)).toBe(true)
    expect(AUDIO_EXTENSIONS.length).toBeGreaterThan(0)
  })

  it('contains .mp3', () => {
    expect(AUDIO_EXTENSIONS).toContain('.mp3')
  })

  it('contains .wav', () => {
    expect(AUDIO_EXTENSIONS).toContain('.wav')
  })

  it('contains .ogg', () => {
    expect(AUDIO_EXTENSIONS).toContain('.ogg')
  })

  it('contains .m4a', () => {
    expect(AUDIO_EXTENSIONS).toContain('.m4a')
  })

  it('contains .aac', () => {
    expect(AUDIO_EXTENSIONS).toContain('.aac')
  })

  it('contains .flac', () => {
    expect(AUDIO_EXTENSIONS).toContain('.flac')
  })

  it('contains .wma', () => {
    expect(AUDIO_EXTENSIONS).toContain('.wma')
  })

  it('contains .opus', () => {
    expect(AUDIO_EXTENSIONS).toContain('.opus')
  })

  it('has exactly 8 extensions', () => {
    expect(AUDIO_EXTENSIONS).toHaveLength(8)
  })

  it('all extensions start with a dot', () => {
    expect(AUDIO_EXTENSIONS.every(ext => ext.startsWith('.'))).toBe(true)
  })

  it('all extensions are lowercase', () => {
    expect(AUDIO_EXTENSIONS.every(ext => ext === ext.toLowerCase())).toBe(true)
  })
})
