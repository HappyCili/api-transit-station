import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

const keysList = vi.hoisted(() => vi.fn())
const listHistory = vi.hoisted(() => vi.fn())
const generate = vi.hoisted(() => vi.fn())
const edit = vi.hoisted(() => vi.fn())
const saveHistory = vi.hoisted(() => vi.fn())
const deleteConversation = vi.hoisted(() => vi.fn())
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
      deleteConversation,
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

function deferred<T>() {
  let resolve!: (value: T) => void
  const promise = new Promise<T>((resolvePromise) => {
    resolve = resolvePromise
  })
  return { promise, resolve }
}

describe('ImageGenerationView', () => {
  beforeEach(() => {
    keysList.mockReset()
    listHistory.mockReset()
    generate.mockReset()
    edit.mockReset()
    saveHistory.mockReset()
    deleteConversation.mockReset().mockResolvedValue({ message: 'ok' })
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
    await vi.waitFor(() => {
      expect(wrapper.get('button[title="imageGeneration.useAsReference"]').attributes('disabled')).toBeUndefined()
    })

    const referenceButton = wrapper.get('button[title="imageGeneration.useAsReference"]')
    await referenceButton.trigger('click')
    await flushPromises()
    await flushPromises()

    expect(wrapper.text()).toContain('imageGeneration.referenceImages')

    await wrapper.get('textarea').setValue('Make a second image using the reference')
    const generateButton = wrapper.findAll('button').find((button) => button.text().includes('imageGeneration.generate'))
    expect(generateButton).toBeTruthy()
    await generateButton!.trigger('click')

    expect((wrapper.get('textarea').element as HTMLTextAreaElement).value).toBe('')
    expect(wrapper.find('[data-testid="reference-image-picker"]').exists()).toBe(false)

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
    expect(wrapper.find('[data-testid="reference-image-picker"]').exists()).toBe(false)
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
    await vi.waitFor(() => {
      expect(wrapper.get('button[title="imageGeneration.useAsReference"]').attributes('disabled')).toBeUndefined()
    })

    const referenceButton = wrapper.get('button[title="imageGeneration.useAsReference"]')
    await referenceButton.trigger('click')
    await flushPromises()
    await flushPromises()

    expect(wrapper.text()).toContain('imageGeneration.referenceImages')

    await wrapper.get('textarea').setValue('Generate from Enter')
    await wrapper.get('textarea').trigger('keydown', { key: 'Enter' })

    expect((wrapper.get('textarea').element as HTMLTextAreaElement).value).toBe('')
    expect(wrapper.find('[data-testid="reference-image-picker"]').exists()).toBe(false)

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
    expect(wrapper.find('[data-testid="reference-image-picker"]').exists()).toBe(false)
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
    expect(wrapper.find('[data-testid="reference-image-picker"]').exists()).toBe(false)
  })

  it('does not show or submit the removed model thinking control', async () => {
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

    expect(wrapper.text()).not.toContain('imageGeneration.modelThinking')
    await wrapper.get('textarea').setValue('No hidden reasoning control')
    const generateButton = wrapper.findAll('button').find((button) => button.text().includes('imageGeneration.generate'))
    expect(generateButton).toBeTruthy()
    await generateButton!.trigger('click')
    await vi.waitFor(() => expect(generate).toHaveBeenCalledTimes(1))

    expect(generate.mock.calls[0][1]).not.toHaveProperty('reasoning_effort')
  })

  it('keeps consecutive generations in one conversation history item', async () => {
    listHistory.mockResolvedValue({
      items: [],
      total: 0,
      page: 1,
      page_size: 50,
    })

    let nextHistoryID = 101
    saveHistory.mockImplementation((payload) => {
      const id = nextHistoryID++
      const conversationID = payload.conversation_id ?? id
      return Promise.resolve({
        ...historyRecord,
        id,
        conversation_id: conversationID,
        conversation_title: conversationID === id ? payload.prompt : 'First prompt',
        turn_index: conversationID === id ? 1 : 2,
        prompt: payload.prompt,
        request: payload.request,
        reference_images: payload.reference_images,
        images: payload.images,
        created_at: `2026-06-22T10:0${id - 100}:00Z`,
        updated_at: `2026-06-22T10:0${id - 100}:00Z`,
      })
    })

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

    const generateButton = wrapper.findAll('button').find((button) => button.text().includes('imageGeneration.generate'))
    expect(generateButton).toBeTruthy()

    await wrapper.get('textarea').setValue('First prompt')
    await generateButton!.trigger('click')
    await vi.waitFor(() => expect(saveHistory).toHaveBeenCalledTimes(1))

    await wrapper.get('textarea').setValue('Second prompt')
    await generateButton!.trigger('click')
    await vi.waitFor(() => expect(saveHistory).toHaveBeenCalledTimes(2))

    expect(saveHistory.mock.calls[0][0]).toEqual(expect.objectContaining({
      conversation_id: null,
      prompt: 'First prompt',
    }))
    expect(saveHistory.mock.calls[1][0]).toEqual(expect.objectContaining({
      conversation_id: 101,
      prompt: 'Second prompt',
    }))

    const conversationButtons = wrapper.findAll('button').filter((button) =>
      button.text().includes('First prompt') && button.text().includes('轮'),
    )
    expect(conversationButtons).toHaveLength(1)
    expect(conversationButtons[0].text()).toContain('2 轮')
  })

  it('deletes every turn in a conversation from one history action', async () => {
    const secondTurn = {
      ...historyRecord,
      id: 11,
      turn_index: 2,
      prompt: 'Turn the cat blue',
      created_at: '2026-06-22T10:01:00Z',
      updated_at: '2026-06-22T10:01:00Z',
    }
    const otherConversation = {
      ...historyRecord,
      id: 20,
      conversation_id: 20,
      conversation_title: 'Landscape study',
      prompt: 'Draw a landscape',
      created_at: '2026-06-22T09:00:00Z',
      updated_at: '2026-06-22T09:00:00Z',
    }
    listHistory.mockResolvedValue({
      items: [secondTurn, historyRecord, otherConversation],
      total: 3,
      page: 1,
      page_size: 50,
    })

    const wrapper = mount(ImageGenerationView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          ConfirmDialog: {
            props: ['show', 'title', 'message', 'confirmText'],
            emits: ['confirm', 'cancel'],
            template: '<div v-if="show" data-testid="confirm-dialog"><span>{{ message }}</span><button data-testid="confirm-delete" @click="$emit(\'confirm\')">{{ confirmText }}</button></div>',
          },
          Icon: { template: '<span />' },
          LoadingSpinner: { template: '<span />' },
        },
      },
    })

    await flushPromises()

    expect(wrapper.findAll('[data-testid="conversation-history-item"]')).toHaveLength(2)
    await wrapper.get('[data-testid="conversation-history-item"][data-conversation-id="10"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-testid="delete-conversation-button"][data-conversation-id="10"]').trigger('click')

    expect(deleteConversation).not.toHaveBeenCalled()
    expect(wrapper.get('[data-testid="confirm-dialog"]').text()).toContain('imageGeneration.deleteConversationConfirm')

    await wrapper.get('[data-testid="confirm-delete"]').trigger('click')
    await vi.waitFor(() => expect(deleteConversation).toHaveBeenCalledWith(10))

    expect(wrapper.find('[data-testid="conversation-history-item"][data-conversation-id="10"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="conversation-history-item"][data-conversation-id="20"]').exists()).toBe(true)
    expect(wrapper.text()).toContain('imageGeneration.welcomeTitle')
    expect(showSuccess).toHaveBeenCalledWith('imageGeneration.deleteConversationSuccess')
  })

  it('shows images from every turn in the selected conversation thumbnail rail', async () => {
    const secondTurn = {
      ...historyRecord,
      id: 11,
      turn_index: 2,
      prompt: 'Turn the cat blue',
      images: [{
        url: 'data:image/png;base64,d29ybGQ=',
        revised_prompt: 'A blue cat',
      }],
      created_at: '2026-06-22T10:01:00Z',
      updated_at: '2026-06-22T10:01:00Z',
    }
    listHistory.mockResolvedValue({
      items: [secondTurn, historyRecord],
      total: 2,
      page: 1,
      page_size: 50,
    })

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

    await vi.waitFor(() => {
      expect(wrapper.findAll('[data-testid="generated-image-thumbnail"] img')).toHaveLength(2)
    })
    const thumbnailImages = wrapper.findAll('[data-testid="generated-image-thumbnail"] img')
    expect(thumbnailImages.map((image) => image.attributes('alt'))).toEqual([
      'Draw a cat with crisp details',
      'A blue cat',
    ])
  })

  it('shows historical reference images in their turn without adding them to the composer', async () => {
    const turnWithReference = {
      ...historyRecord,
      reference_images: [imageDataURL],
    }
    listHistory.mockResolvedValue({
      items: [turnWithReference],
      total: 1,
      page: 1,
      page_size: 50,
    })

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

    await wrapper.get('[data-testid="conversation-history-item"][data-conversation-id="10"]').trigger('click')
    await flushPromises()

    const historicalReferences = wrapper.get('[data-testid="conversation-reference-images"]')
    expect(historicalReferences.text()).toContain('imageGeneration.referenceImages')
    expect(historicalReferences.find('img').attributes('src')).toBe(imageDataURL)
    expect(wrapper.find('[data-testid="reference-image-picker"]').exists()).toBe(false)
  })

  it('does not let a stale preview refresh replace newer blob URLs', async () => {
    const firstTurn = {
      ...historyRecord,
      images: [{ url: '/storage/api-images/first.webp', revised_prompt: 'First image' }],
    }
    const secondTurn = {
      ...historyRecord,
      id: 11,
      turn_index: 2,
      prompt: 'Second turn',
      images: [{ url: '/storage/api-images/second.webp', revised_prompt: 'Second image' }],
      created_at: '2026-06-22T10:01:00Z',
      updated_at: '2026-06-22T10:01:00Z',
    }
    const historyResponse = {
      items: [secondTurn, firstTurn],
      total: 2,
      page: 1,
      page_size: 50,
    }
    const detailResponse = deferred<typeof historyResponse>()
    listHistory.mockReset()
      .mockResolvedValueOnce(historyResponse)
      .mockReturnValueOnce(detailResponse.promise)

    const oldFirstBlob = new Blob(['old-first'], { type: 'image/png' })
    const oldSecondBlob = new Blob(['old-second'], { type: 'image/png' })
    const newFirstBlob = new Blob(['new-first'], { type: 'image/png' })
    const newSecondBlob = new Blob(['new-second'], { type: 'image/png' })
    const previewResponses = [
      deferred<Blob>(),
      deferred<Blob>(),
      deferred<Blob>(),
      deferred<Blob>(),
    ]
    let previewResponseIndex = 0
    viewHistoryImage.mockImplementation(() => previewResponses[previewResponseIndex++].promise)

    const objectURLs = new Map<Blob, string>([
      [oldFirstBlob, 'blob:old-first'],
      [oldSecondBlob, 'blob:old-second'],
      [newFirstBlob, 'blob:new-first'],
      [newSecondBlob, 'blob:new-second'],
    ])
    createObjectURL.mockImplementation((blob: Blob) => objectURLs.get(blob) || 'blob:unknown')

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
    await vi.waitFor(() => expect(viewHistoryImage).toHaveBeenCalledTimes(2))

    previewResponses[0].resolve(oldFirstBlob)
    await vi.waitFor(() => expect(createObjectURL).toHaveBeenCalledWith(oldFirstBlob))

    detailResponse.resolve(historyResponse)
    await vi.waitFor(() => expect(viewHistoryImage).toHaveBeenCalledTimes(4))

    previewResponses[2].resolve(newFirstBlob)
    previewResponses[3].resolve(newSecondBlob)
    await vi.waitFor(() => {
      const sources = wrapper.findAll('[data-testid="generated-image-thumbnail"] img').map((image) => image.attributes('src'))
      expect(sources).toEqual(['blob:new-first', 'blob:new-second'])
    })

    previewResponses[1].resolve(oldSecondBlob)
    await vi.waitFor(() => {
      expect(revokeObjectURL).toHaveBeenCalledWith('blob:old-first')
      expect(revokeObjectURL).toHaveBeenCalledWith('blob:old-second')
      const sources = wrapper.findAll('[data-testid="generated-image-thumbnail"] img').map((image) => image.attributes('src'))
      expect(sources).toEqual(['blob:new-first', 'blob:new-second'])
    })
  })
})
