import axios, { AxiosError } from 'axios'
import { apiClient } from './client'
import type { PaginatedResponse } from '@/types'

const OPENAI_BASE_URL = import.meta.env.VITE_OPENAI_COMPAT_BASE_URL || '/v1'

const openAIImageClient = axios.create({
  baseURL: OPENAI_BASE_URL,
  timeout: 300000,
})

const IMAGE_GENERATION_REQUEST_TIMEOUT_MS = 0

export type ImageAspectRatio = '1:1' | '16:9' | '9:16' | '4:3' | '3:4'
export type ImageQuality = 'low' | 'medium' | 'high'
export type ImageGenerationRequestSize = `${number}x${number}`
export type ImageOutputFormat = 'webp' | 'png' | 'jpeg'
export type ImageStyle = 'vivid' | 'natural'
export type ImageBackground = 'auto' | 'opaque' | 'transparent'
export type ImageReasoningEffort = 'low' | 'medium' | 'high'

const IMAGE_GENERATION_SIZE_MAP: Record<ImageQuality, Record<ImageAspectRatio, ImageGenerationRequestSize>> = {
  low: {
    '1:1': '1024x1024',
    '16:9': '1024x576',
    '9:16': '576x1024',
    '4:3': '1024x768',
    '3:4': '768x1024',
  },
  medium: {
    '1:1': '2048x2048',
    '16:9': '2048x1152',
    '9:16': '1152x2048',
    '4:3': '2048x1536',
    '3:4': '1536x2048',
  },
  high: {
    '1:1': '2160x2160',
    '16:9': '3840x2160',
    '9:16': '2160x3840',
    '4:3': '2880x2160',
    '3:4': '2160x2880',
  },
}

export interface OpenAIImageData {
  url?: string
  b64_json?: string
  revised_prompt?: string | null
}

export interface OpenAIImageResponse {
  created: number
  data: OpenAIImageData[]
  model?: string
  size?: string
  quality?: string
  output_format?: string
  background?: string
}

export interface ImageGenerationPayload {
  model: string
  prompt: string
  n: number
  size: ImageGenerationRequestSize
  quality: ImageQuality
  response_format: 'url' | 'b64_json'
  style: ImageStyle
  background: ImageBackground
  output_format: ImageOutputFormat
  output_compression?: number
  moderation: 'auto' | 'low'
  reasoning_effort?: ImageReasoningEffort
  reference_images?: string[]
}

export interface ImageGenerationHistoryRecord {
  id: number
  conversation_id: number
  conversation_title: string
  turn_index: number
  user_id: number
  api_key_id: number | null
  prompt: string
  revised_prompt: string | null
  model: string
  size: string
  quality: string
  output_format: string
  n: number
  request: Record<string, unknown>
  reference_images: string[]
  images: OpenAIImageData[]
  status: 'succeeded' | 'failed'
  error_message: string | null
  created_at: string
  updated_at: string
}

export interface SaveImageGenerationHistoryPayload {
  api_key_id?: number | null
  conversation_id?: number | null
  conversation_title?: string
  prompt: string
  revised_prompt?: string | null
  model: string
  size: string
  quality: string
  output_format: string
  n: number
  request: Record<string, unknown>
  reference_images: string[]
  images: OpenAIImageData[]
  status: 'succeeded' | 'failed'
  error_message?: string | null
}

export interface ImageGenerationHistoryFilters {
  page?: number
  page_size?: number
  conversation_id?: number
  status?: string
  search?: string
}

function authHeaders(apiKey: string): Record<string, string> {
  return { Authorization: `Bearer ${apiKey}` }
}

export function resolveImageGenerationRequestSize(
  aspectRatio: ImageAspectRatio,
  quality: ImageQuality,
): ImageGenerationRequestSize {
  return IMAGE_GENERATION_SIZE_MAP[quality][aspectRatio]
}

export function extractOpenAIImageError(error: unknown, fallback = 'Image request failed'): string {
  if (axios.isAxiosError(error)) {
    const axiosError = error as AxiosError<{ error?: { message?: string }; message?: string }>
    return (
      axiosError.response?.data?.error?.message ||
      axiosError.response?.data?.message ||
      axiosError.message ||
      fallback
    )
  }
  return error instanceof Error ? error.message : fallback
}

export function isOpenAIImageRequestTimeout(error: unknown): boolean {
  if (!axios.isAxiosError(error)) {
    return false
  }
  const message = (error.message || '').toLowerCase()
  return error.code === 'ECONNABORTED' || message.includes('timeout') || message.includes('timed out')
}

async function listModels(apiKey: string): Promise<{ data: Array<{ id: string; object: string; owned_by?: string; kind?: string }> }> {
  const { data } = await openAIImageClient.get('/models', { headers: authHeaders(apiKey) })
  return data
}

async function generate(apiKey: string, payload: ImageGenerationPayload): Promise<OpenAIImageResponse> {
  const { data } = await openAIImageClient.post<OpenAIImageResponse>('/images/generations', payload, {
    headers: { ...authHeaders(apiKey), 'Content-Type': 'application/json' },
    timeout: IMAGE_GENERATION_REQUEST_TIMEOUT_MS,
  })
  return data
}

async function edit(apiKey: string, form: FormData): Promise<OpenAIImageResponse> {
  const { data } = await openAIImageClient.post<OpenAIImageResponse>('/images/edits', form, {
    headers: authHeaders(apiKey),
    timeout: IMAGE_GENERATION_REQUEST_TIMEOUT_MS,
  })
  return data
}

async function listHistory(filters: ImageGenerationHistoryFilters = {}): Promise<PaginatedResponse<ImageGenerationHistoryRecord>> {
  const { data } = await apiClient.get<PaginatedResponse<ImageGenerationHistoryRecord>>('/user/image-generations', {
    params: filters,
  })
  return data
}

async function saveHistory(payload: SaveImageGenerationHistoryPayload): Promise<ImageGenerationHistoryRecord> {
  const { data } = await apiClient.post<ImageGenerationHistoryRecord>('/user/image-generations', payload)
  return data
}

async function deleteHistory(id: number): Promise<{ message: string }> {
  const { data } = await apiClient.delete<{ message: string }>(`/user/image-generations/${id}`)
  return data
}

async function downloadHistoryImage(id: number, index: number): Promise<Blob> {
  const { data } = await apiClient.get<Blob>(`/user/image-generations/${id}/images/${index}/download`, {
    responseType: 'blob',
  })
  return data
}

async function viewHistoryImage(id: number, index: number): Promise<Blob> {
  const { data } = await apiClient.get<Blob>(`/user/image-generations/${id}/images/${index}/view`, {
    responseType: 'blob',
  })
  return data
}

export const imageGenerationAPI = {
  listModels,
  generate,
  edit,
  listHistory,
  saveHistory,
  deleteHistory,
  downloadHistoryImage,
  viewHistoryImage,
}

export default imageGenerationAPI
