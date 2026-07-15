import { beforeEach, describe, expect, it, vi } from 'vitest'

const deleteRequest = vi.hoisted(() => vi.fn())

vi.mock('@/api/client', () => ({
  apiClient: {
    delete: deleteRequest,
  },
}))

import {
  imageGenerationAPI,
  resolveImageGenerationRequestSize,
  type ImageAspectRatio,
  type ImageQuality,
} from '@/api/imageGeneration'

describe('resolveImageGenerationRequestSize', () => {
  beforeEach(() => {
    deleteRequest.mockReset()
  })
  const cases: Array<[ImageQuality, ImageAspectRatio, string]> = [
    ['low', '1:1', '1024x1024'],
    ['low', '16:9', '1024x576'],
    ['low', '9:16', '576x1024'],
    ['low', '4:3', '1024x768'],
    ['low', '3:4', '768x1024'],
    ['medium', '1:1', '2048x2048'],
    ['medium', '16:9', '2048x1152'],
    ['medium', '9:16', '1152x2048'],
    ['medium', '4:3', '2048x1536'],
    ['medium', '3:4', '1536x2048'],
    ['high', '1:1', '2160x2160'],
    ['high', '16:9', '3840x2160'],
    ['high', '9:16', '2160x3840'],
    ['high', '4:3', '2880x2160'],
    ['high', '3:4', '2160x2880'],
  ]

  it.each(cases)('maps %s %s to %s', (quality, ratio, expected) => {
    expect(resolveImageGenerationRequestSize(ratio, quality)).toBe(expected)
  })

  it('always returns WIDTHxHEIGHT values aligned to the image API size constraints', () => {
    for (const [quality, ratio] of cases) {
      const value = resolveImageGenerationRequestSize(ratio, quality)
      const match = /^(\d+)x(\d+)$/.exec(value)

      expect(match).not.toBeNull()
      const width = Number(match?.[1])
      const height = Number(match?.[2])
      expect(width % 16).toBe(0)
      expect(height % 16).toBe(0)
      expect(width).toBeGreaterThanOrEqual(512)
      expect(height).toBeGreaterThanOrEqual(512)
      expect(width).toBeLessThanOrEqual(4096)
      expect(height).toBeLessThanOrEqual(4096)
    }
  })

  it('deletes history by conversation ID', async () => {
    deleteRequest.mockResolvedValue({ data: { message: 'ok' } })

    await imageGenerationAPI.deleteConversation(42)

    expect(deleteRequest).toHaveBeenCalledWith('/user/image-generations', {
      params: { conversation_id: 42 },
    })
  })
})
