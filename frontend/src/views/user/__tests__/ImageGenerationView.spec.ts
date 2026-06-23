import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

const keysList = vi.hoisted(() => vi.fn())
const listHistory = vi.hoisted(() => vi.fn())
const generate = vi.hoisted(() => vi.fn())
const edit = vi.hoisted(() => vi.fn())
const saveHistory = vi.hoisted(() => vi.fn())
const viewHistoryImage = vi.hoisted(() => vi.fn())
const downloadHistoryImage = vi.hoisted(() => vi.fn())
const showSuccess = vi.hoisted(() => vi.fn())
const showError = vi.hoisted(() => vi.fn())
const createObjectURL = vi.hoisted(() => vi.fn())
const revokeObjectURL = vi.hoisted(() => vi.fn())

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key,
    }),
  }
})

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showSuccess,
    showError,
  }),
}))

vi.mock('@/api/keys', () => ({
  default: {
    list: keysList,
  },
}))

vi.mock('@/api/imageGeneration', async () => {
  const actual = await vi.importActual<typeof import('@/api/imageGeneration')>('@/api/imageGeneration')
  return {
    ...actual,
    default: {
      ...actual.default,
      generate,
      edit,
      listHistory,
      saveHistory,
      viewHistoryImage,
      downloadHistoryImage,
    },
  }
})

import ImageGenerationView from '../ImageGenerationView.vue'

const imageDataURL = 'data:image/png;base64,aGVsbG8='

const historyRecord = {
  id: 10,
  conversation_id: 10,
  conversation_title: 'Cat study',
  turn_index: 1,
  user_id: 7,
  api_key_id: 1,
  prompt: 'Draw a cat',
  revised_prompt: null,
  model: 'gpt-image-2',
  size: '1:1',
  quality: 'high',
  output_format: 'png',
  n: 1,
  request: {},
  reference_images: [],
  images: [
    {
      url: imageDataURL,
      revised_prompt: 'Draw a cat with crisp details',
    },
  ],
  favorite: false,
  status: 'succeeded',
  error_message: null,
  created_at: '2026-06-22T10:00:00Z',
  updated_at: '2026-06-22T10:00:00Z',
}

describe('ImageGenerationView', () => {
  beforeEach(() => {
    keysList.mockReset()
    listHistory.mockReset()
    generate.mockReset()
    edit.mockReset()
    saveHistory.mockReset()
    viewHistoryImage.mockReset()
    downloadHistoryImage.mockReset()
    showSuccess.mockReset()
    showError.mockReset()
    createObjectURL.mockReset().mockReturnValue('blob:preview')
    revokeObjectURL.mockReset()

    Object.defineProperty(URL, 'createObjectURL', {
      configurable: true,
      value: createObjectURL,
    })
    Object.defineProperty(URL, 'revokeObjectURL', {
      configurable: true,
      value: revokeObjectURL,
    })

    keysList.mockResolvedValue({
      items: [
        {
          id: 1,
          key: 'sk-test',
          name: 'Image key',
          status: 'active',
          group: {
            name: 'OpenAI images',
            platform: 'openai',
            allow_image_generation: true,
          },
        },
      ],
    })
    listHistory.mockResolvedValue({
      items: [historyRecord],
      total: 1,
      page: 1,
      page_size: 50,
    })
    generate.mockResolvedValue({
      created: 1782100000,
      data: [
        {
          b64_json: 'cmVzdWx0',
          revised_prompt: 'A variant of the cat',
        },
      ],
    })
    edit.mockResolvedValue({
      created: 1782100000,
      data: [
        {
          b64_json: 'cmVzdWx0',
          revised_prompt: 'A variant of the cat',
        },
      ],
    })
    saveHistory.mockImplementation((payload) => Promise.resolve({
      ...historyRecord,
      id: 11,
      prompt: payload.prompt,
      request: payload.request,
      reference_images: payload.reference_images,
      images: payload.images,
      created_at: '2026-06-22T10:01:00Z',
      updated_at: '2026-06-22T10:01:00Z',
    }))
  })

  it('edits from a generated image reference with multipart image input', async () => {
    const wrapper = mount(ImageGenerationView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          Icon: { template: '<span />' },
          LoadingSpinner: { template: '<span />' },
        },
      },
    })

    await flushPromises()

    const historyButton = wrapper.findAll('button').find((button) => button.text().includes('Cat study'))
    expect(historyButton).toBeTruthy()
    await historyButton!.trigger('click')
    await flushPromises()
    await flushPromises()

    const referenceButton = wrapper.get('button[title="imageGeneration.useAsReference"]')
    expect(referenceButton.attributes('disabled')).toBeUndefined()
    await referenceButton.trigger('click')
    await flushPromises()
    await flushPromises()

    expect(wrapper.text()).toContain('imageGeneration.referenceImages')

    await wrapper.get('textarea').setValue('Make a second image using the reference')
    const generateButton = wrapper.findAll('button').find((button) => button.text().includes('imageGeneration.generate'))
    expect(generateButton).toBeTruthy()
    await generateButton!.trigger('click')

    expect((wrapper.get('textarea').element as HTMLTextAreaElement).value).toBe('')
    expect(wrapper.text()).not.toContain('imageGeneration.referenceImages')

    await flushPromises()

    expect(generate).not.toHaveBeenCalled()
    expect(edit).toHaveBeenCalledWith('sk-test', expect.any(FormData))
    const form = edit.mock.calls[0][1] as FormData
    expect(form.get('prompt')).toBe('Make a second image using the reference')
    expect(form.get('model')).toBe('gpt-image-2')
    expect(form.get('image')).toBeInstanceOf(Blob)
    expect((form.get('image') as Blob).type).toBe('image/png')
    expect(saveHistory).toHaveBeenCalledWith(expect.objectContaining({
      reference_images: [imageDataURL],
    }))
    expect((wrapper.get('textarea').element as HTMLTextAreaElement).value).toBe('')
    expect(wrapper.text()).not.toContain('imageGeneration.referenceImages')
  })

  it('submits with Enter and clears the prompt and reference image picker', async () => {
    const wrapper = mount(ImageGenerationView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          Icon: { template: '<span />' },
          LoadingSpinner: { template: '<span />' },
        },
      },
    })

    await flushPromises()

    const historyButton = wrapper.findAll('button').find((button) => button.text().includes('Cat study'))
    expect(historyButton).toBeTruthy()
    await historyButton!.trigger('click')
    await flushPromises()
    await flushPromises()

    const referenceButton = wrapper.get('button[title="imageGeneration.useAsReference"]')
    await referenceButton.trigger('click')
    await flushPromises()
    await flushPromises()

    expect(wrapper.text()).toContain('imageGeneration.referenceImages')

    await wrapper.get('textarea').setValue('Generate from Enter')
    await wrapper.get('textarea').trigger('keydown', { key: 'Enter' })

    expect((wrapper.get('textarea').element as HTMLTextAreaElement).value).toBe('')
    expect(wrapper.text()).not.toContain('imageGeneration.referenceImages')

    await flushPromises()

    expect(generate).not.toHaveBeenCalled()
    expect(edit).toHaveBeenCalledWith('sk-test', expect.any(FormData))
    const form = edit.mock.calls[0][1] as FormData
    expect(form.get('prompt')).toBe('Generate from Enter')
    expect(form.get('image')).toBeInstanceOf(Blob)
    expect(saveHistory).toHaveBeenCalledWith(expect.objectContaining({
      prompt: 'Generate from Enter',
      reference_images: [imageDataURL],
    }))
    expect((wrapper.get('textarea').element as HTMLTextAreaElement).value).toBe('')
    expect(wrapper.text()).not.toContain('imageGeneration.referenceImages')
  })

  it('adds uploaded images as references and submits through multipart edit', async () => {
    const wrapper = mount(ImageGenerationView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          Icon: { template: '<span />' },
          LoadingSpinner: { template: '<span />' },
        },
      },
    })

    await flushPromises()

    const fileInput = wrapper.get('input[type="file"]')
    const files = [
      new File(['local-one'], 'reference-one.png', { type: 'image/png' }),
      new File(['local-two'], 'reference-two.jpeg', { type: 'image/jpeg' }),
    ]
    Object.defineProperty(fileInput.element, 'files', {
      configurable: true,
      get: () => files,
    })
    await fileInput.trigger('change')

    await vi.waitFor(() => {
      expect(wrapper.text()).toContain('imageGeneration.referenceImages')
    })

    await wrapper.get('textarea').setValue('Use the uploaded references')
    const generateButton = wrapper.findAll('button').find((button) => button.text().includes('imageGeneration.generate'))
    expect(generateButton).toBeTruthy()
    await generateButton!.trigger('click')
    await flushPromises()

    expect(generate).not.toHaveBeenCalled()
    expect(edit).toHaveBeenCalledWith('sk-test', expect.any(FormData))
    const form = edit.mock.calls[0][1] as FormData
    expect(form.get('prompt')).toBe('Use the uploaded references')
    expect(form.get('image')).toBeNull()

    const uploadedImages = form.getAll('image[]')
    expect(uploadedImages).toHaveLength(2)
    expect(uploadedImages[0]).toBeInstanceOf(Blob)
    expect((uploadedImages[0] as Blob).type).toBe('image/png')
    expect((uploadedImages[1] as Blob).type).toBe('image/jpeg')
    expect(saveHistory).toHaveBeenCalledWith(expect.objectContaining({
      prompt: 'Use the uploaded references',
      reference_images: [
        expect.stringMatching(/^data:image\/png;base64,/),
        expect.stringMatching(/^data:image\/jpeg;base64,/),
      ],
    }))
    expect(wrapper.text()).not.toContain('imageGeneration.referenceImages')
  })
})
