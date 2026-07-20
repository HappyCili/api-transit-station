<template>
  <AppLayout>
    <div class="flex min-h-[calc(100dvh-6rem)] min-w-0 flex-col gap-4 md:h-[calc(100dvh-7rem)] md:min-h-0 md:flex-row md:items-stretch md:overflow-hidden lg:h-[calc(100dvh-8rem)]">
      <aside class="flex h-[clamp(10rem,32dvh,18rem)] w-full flex-none flex-col overflow-hidden rounded-lg border border-gray-100 bg-white dark:border-dark-700 dark:bg-dark-800 sm:h-[clamp(12rem,36dvh,22rem)] md:h-full md:w-52 lg:w-64">
        <div class="flex-none border-b border-gray-100 p-4 dark:border-dark-700">
          <button class="btn btn-primary w-full justify-center" @click="startNewConversation">
            <Icon name="plus" size="sm" class="mr-2" />
            {{ t('imageGeneration.newConversation') }}
          </button>
        </div>

        <div class="min-h-0 flex-1 overflow-y-auto overscroll-contain p-3">
          <div v-if="historyLoading" class="flex justify-center py-8">
            <LoadingSpinner size="md" />
          </div>
          <div v-else-if="historyConversations.length === 0" class="px-3 py-8 text-center text-sm text-gray-500 dark:text-gray-400">
            {{ t('imageGeneration.emptyHistory') }}
          </div>
          <div
            v-for="item in historyConversations"
            :key="item.conversationId"
            class="group relative mb-2 rounded-lg border border-transparent transition-colors hover:border-gray-200 hover:bg-gray-50 dark:hover:border-dark-600 dark:hover:bg-dark-700/60"
          >
            <button
              type="button"
              data-testid="conversation-history-item"
              :data-conversation-id="item.conversationId"
              class="w-full rounded-lg p-3 pr-11 text-left"
              @click="selectConversation(item)"
            >
              <div class="flex items-start justify-between gap-2">
                <div class="min-w-0">
                  <p class="truncate text-sm font-medium text-gray-900 dark:text-white">{{ item.title }}</p>
                  <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ formatDate(item.updatedAt) }}</p>
                </div>
              </div>
              <div class="mt-2 flex flex-wrap items-center gap-2 text-xs text-gray-500 dark:text-gray-400">
                <span>{{ item.turnCount }} 轮</span>
                <span>{{ item.latest.model }}</span>
                <span>{{ item.latest.size }}</span>
                <span>{{ item.latest.quality }}</span>
                <span
                  v-if="isUncertainHistoryRecord(item.latest)"
                  class="rounded-full bg-amber-50 px-2 py-0.5 font-medium text-amber-700 dark:bg-amber-900/30 dark:text-amber-300"
                >
                  {{ t('imageGeneration.statusUncertain') }}
                </span>
                <span
                  v-else-if="item.latest.status === 'failed'"
                  class="rounded-full bg-red-50 px-2 py-0.5 font-medium text-red-600 dark:bg-red-900/30 dark:text-red-300"
                >
                  {{ t('imageGeneration.statusFailed') }}
                </span>
              </div>
              <p v-if="isUncertainHistoryRecord(item.latest)" class="mt-2 break-words text-xs text-amber-700 dark:text-amber-300">
                {{ t('imageGeneration.generateTimeoutUncertain') }}
              </p>
              <p v-else-if="item.latest.status === 'failed' && item.latest.error_message" class="mt-2 break-words text-xs text-red-600 dark:text-red-300">
                {{ item.latest.error_message }}
              </p>
            </button>
            <button
              type="button"
              data-testid="delete-conversation-button"
              :data-conversation-id="item.conversationId"
              :title="t('imageGeneration.deleteConversation')"
              :aria-label="t('imageGeneration.deleteConversation')"
              :disabled="conversationDeletePending || (generating && currentConversationId === item.conversationId)"
              class="absolute right-2 top-2 flex h-8 w-8 items-center justify-center rounded-md text-gray-400 transition-colors hover:bg-red-50 hover:text-red-600 focus:outline-none focus-visible:ring-2 focus-visible:ring-red-500 disabled:cursor-not-allowed disabled:opacity-40 dark:text-gray-500 dark:hover:bg-red-950/40 dark:hover:text-red-300"
              @click="requestDeleteConversation(item)"
            >
              <Icon name="trash" size="sm" />
            </button>
          </div>
        </div>
      </aside>

      <main class="flex min-h-0 min-w-0 flex-1 flex-col gap-4 md:h-full">
        <section class="flex min-h-0 flex-1 flex-col overflow-hidden rounded-lg border border-gray-100 bg-white p-4 dark:border-dark-700 dark:bg-dark-800">
          <div class="flex min-h-0 flex-1 flex-col gap-4 md:flex-row">
            <div class="flex min-h-0 min-w-0 flex-1 flex-col gap-4 lg:flex-row">
              <aside class="flex flex-col gap-2 rounded-lg border border-gray-100 bg-gray-50 p-3 dark:border-dark-700 dark:bg-dark-900/40 lg:w-24 lg:flex-shrink-0">
                <div class="flex items-center justify-between gap-2 lg:block">
                  <p class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('imageGeneration.generatedImages') }}</p>
                  <span class="text-xs text-gray-400 dark:text-gray-500">{{ currentImageEntries.length }}</span>
                </div>
                <div v-if="currentImageEntries.length > 0" class="flex gap-2 overflow-x-auto pb-1 lg:max-h-[520px] lg:flex-col lg:overflow-y-auto lg:pb-0">
                  <button
                    v-for="(entry, entryPosition) in currentImageEntries"
                    :key="entry.key"
                    type="button"
                    data-testid="generated-image-thumbnail"
                    class="h-16 w-16 flex-shrink-0 overflow-hidden rounded-lg border bg-white transition-colors dark:bg-dark-800"
                    :class="selectedImageIndex === entryPosition ? 'border-primary-500 ring-2 ring-primary-100 dark:ring-primary-900/60' : 'border-gray-200 hover:border-primary-300 dark:border-dark-600'"
                    @click="selectGeneratedImage(entryPosition)"
                  >
                    <img v-if="entry.src" :src="entry.src" :alt="entry.prompt" class="h-full w-full object-cover" />
                    <div v-else class="flex h-full w-full items-center justify-center">
                      <LoadingSpinner v-if="entry.loading" size="sm" />
                      <Icon v-else name="x" size="sm" class="text-gray-400 dark:text-gray-500" />
                    </div>
                  </button>
                </div>
                <div v-else class="flex h-16 items-center justify-center rounded-lg border border-dashed border-gray-200 px-2 text-center text-xs text-gray-400 dark:border-dark-600 dark:text-gray-500">
                  {{ t('imageGeneration.noGeneratedImages') }}
                </div>
              </aside>

              <div class="min-h-64 min-w-0 flex-1 overflow-y-auto rounded-lg border border-gray-100 bg-gray-50 p-4 md:h-full md:min-h-0 dark:border-dark-700 dark:bg-dark-900/40">
                <div class="space-y-5">
                  <template v-if="conversationTurns.length > 0">
                    <template v-for="turn in conversationTurns" :key="turn.id">
                      <div class="flex justify-end">
                        <div class="max-w-[min(42rem,88%)] rounded-lg rounded-tr-sm bg-primary-600 px-4 py-3 text-white shadow-sm">
                          <div class="mb-1 text-xs font-medium text-primary-100">{{ t('imageGeneration.userLabel') }}</div>
                          <p class="whitespace-pre-wrap break-words text-sm leading-6">{{ turn.prompt }}</p>
                          <div
                            v-if="turnReferenceImageEntries(turn).length > 0"
                            data-testid="conversation-reference-images"
                            class="mt-3 border-t border-white/20 pt-3"
                          >
                            <div class="mb-2 flex items-center gap-2 text-xs font-medium text-primary-100">
                              <Icon name="link" size="xs" />
                              <span>{{ t('imageGeneration.referenceImages') }}</span>
                              <span class="rounded-full bg-white/15 px-1.5 py-0.5">{{ turnReferenceImageEntries(turn).length }}</span>
                            </div>
                            <div class="flex gap-2 overflow-x-auto pb-1">
                              <img
                                v-for="entry in turnReferenceImageEntries(turn)"
                                :key="entry.key"
                                :src="entry.dataUrl"
                                :alt="entry.prompt"
                                class="h-14 w-14 flex-shrink-0 rounded-md border border-white/20 object-cover"
                              />
                            </div>
                          </div>
                          <div class="mt-2 flex flex-wrap gap-2 text-xs text-primary-100">
                            <span>{{ turn.model }}</span>
                            <span>{{ turn.size }}</span>
                            <span>{{ turn.quality }}</span>
                          </div>
                        </div>
                      </div>

                      <div class="flex justify-start">
                        <div class="max-w-[min(48rem,96%)] rounded-lg rounded-tl-sm border border-gray-100 bg-white px-4 py-3 shadow-sm dark:border-dark-700 dark:bg-dark-800">
                          <div class="mb-3 text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('imageGeneration.assistantLabel') }} · #{{ turn.turn_index }}</div>

                          <div v-if="isUncertainHistoryRecord(turn)" class="flex gap-3 rounded-lg border border-amber-100 bg-amber-50 p-3 dark:border-amber-900/60 dark:bg-amber-950/20">
                            <div class="flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-lg bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300">
                              <Icon name="infoCircle" size="sm" />
                            </div>
                            <div class="min-w-0">
                              <h2 class="text-sm font-semibold text-amber-900 dark:text-amber-100">{{ t('imageGeneration.resultUncertainTitle') }}</h2>
                              <p class="mt-1 break-words text-sm leading-6 text-amber-800 dark:text-amber-200">{{ t('imageGeneration.generateTimeoutUncertain') }}</p>
                            </div>
                          </div>

                          <div v-else-if="turn.status === 'failed'" class="flex gap-3 rounded-lg border border-red-100 bg-red-50 p-3 dark:border-red-900/60 dark:bg-red-950/20">
                            <div class="flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-lg bg-red-100 text-red-600 dark:bg-red-900/40 dark:text-red-300">
                              <Icon name="x" size="sm" />
                            </div>
                            <div class="min-w-0">
                              <h2 class="text-sm font-semibold text-red-900 dark:text-red-100">{{ t('imageGeneration.failedTitle') }}</h2>
                              <p class="mt-1 break-words text-sm leading-6 text-red-700 dark:text-red-200">{{ turn.error_message || t('imageGeneration.generateFailed') }}</p>
                              <button
                                type="button"
                                data-testid="retry-failed-generation-button"
                                class="mt-3 inline-flex items-center gap-1.5 rounded-md border border-red-200 bg-white px-2.5 py-1.5 text-sm font-medium text-red-700 transition-colors hover:bg-red-100 disabled:cursor-not-allowed disabled:opacity-50 dark:border-red-900/70 dark:bg-dark-800 dark:text-red-300 dark:hover:bg-red-950/40"
                                :disabled="generating"
                                @click="retryFailedTurn(turn)"
                              >
                                <Icon name="refresh" size="sm" :class="generating ? 'animate-spin' : ''" />
                                {{ t('imageGeneration.retry') }}
                              </button>
                            </div>
                          </div>

                          <div v-else class="space-y-3">
                            <div
                              v-for="entry in turnImageEntries(turn)"
                              :key="entry.key"
                              class="overflow-hidden rounded-lg border border-gray-100 bg-gray-100 dark:border-dark-700 dark:bg-dark-900"
                            >
                              <img v-if="entry.src" :src="entry.src" :alt="entry.prompt" class="max-h-[560px] w-full object-contain" />
                              <div v-else-if="entry.loading" class="flex min-h-64 items-center justify-center">
                                <LoadingSpinner size="md" />
                              </div>
                              <div v-else class="flex min-h-64 flex-col items-center justify-center gap-2 px-4 text-center text-sm text-gray-500 dark:text-gray-400">
                                <Icon name="x" size="sm" />
                                <span>{{ t('imageGeneration.imageLoadFailed') }}</span>
                              </div>
                              <div class="flex items-center justify-between gap-3 border-t border-gray-100 bg-white px-3 py-2 dark:border-dark-700 dark:bg-dark-800">
                                <span class="min-w-0 truncate text-sm text-gray-500 dark:text-gray-400">{{ entry.prompt }}</span>
                                <div class="flex flex-shrink-0 items-center gap-1">
                                  <button type="button" class="rounded-md p-1.5 text-gray-500 hover:bg-gray-100 hover:text-gray-700 disabled:cursor-not-allowed disabled:opacity-50 dark:hover:bg-dark-700 dark:hover:text-gray-200" :disabled="entry.loading || entry.failed" :title="t('imageGeneration.useAsReference')" @click="addReferenceImage(entry)">
                                    <Icon name="link" size="sm" />
                                  </button>
                                  <button type="button" class="rounded-md p-1.5 text-gray-500 hover:bg-gray-100 hover:text-gray-700 dark:hover:bg-dark-700 dark:hover:text-gray-200" :title="t('imageGeneration.copyImage')" @click="copyImage(entry.image, entry.index, turn.id)">
                                    <Icon name="copy" size="sm" />
                                  </button>
                                  <button type="button" class="rounded-md p-1.5 text-gray-500 hover:bg-gray-100 hover:text-gray-700 dark:hover:bg-dark-700 dark:hover:text-gray-200" :title="t('imageGeneration.download')" @click="downloadImage(entry.image, entry.index, turn.id)">
                                    <Icon name="download" size="sm" />
                                  </button>
                                </div>
                              </div>
                            </div>
                          </div>
                        </div>
                      </div>
                    </template>

                    <div v-if="generating && conversationHasPrompt" class="flex justify-end">
                      <div class="max-w-[min(42rem,88%)] rounded-lg rounded-tr-sm bg-primary-600 px-4 py-3 text-white shadow-sm">
                        <div class="mb-1 text-xs font-medium text-primary-100">{{ t('imageGeneration.userLabel') }}</div>
                        <p class="whitespace-pre-wrap break-words text-sm leading-6">{{ submittedPrompt }}</p>
                        <div class="mt-2 flex flex-wrap gap-2 text-xs text-primary-100">
                          <span>{{ submittedModel }}</span>
                          <span>{{ submittedSize }}</span>
                          <span>{{ submittedQuality }}</span>
                        </div>
                      </div>
                    </div>

                    <div v-if="generating" class="flex justify-start">
                      <div class="max-w-[min(48rem,96%)] rounded-lg rounded-tl-sm border border-gray-100 bg-white px-4 py-3 shadow-sm dark:border-dark-700 dark:bg-dark-800">
                        <div class="mb-3 text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('imageGeneration.assistantLabel') }}</div>
                        <div class="flex min-h-40 items-center gap-3 text-sm text-gray-500 dark:text-gray-400">
                          <LoadingSpinner size="md" />
                          <span>{{ t('imageGeneration.generating') }}</span>
                        </div>
                      </div>
                    </div>

                    <div v-if="currentWarningMessage && !generating" class="flex justify-start">
                      <div class="max-w-[min(48rem,96%)] rounded-lg rounded-tl-sm border border-amber-100 bg-white px-4 py-3 shadow-sm dark:border-amber-900/50 dark:bg-dark-800">
                        <div class="mb-3 text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('imageGeneration.assistantLabel') }}</div>
                        <div class="flex gap-3 rounded-lg border border-amber-100 bg-amber-50 p-3 dark:border-amber-900/60 dark:bg-amber-950/20">
                          <div class="flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-lg bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300">
                            <Icon name="infoCircle" size="sm" />
                          </div>
                          <div class="min-w-0">
                            <h2 class="text-sm font-semibold text-amber-900 dark:text-amber-100">{{ t('imageGeneration.resultUncertainTitle') }}</h2>
                            <p class="mt-1 break-words text-sm leading-6 text-amber-800 dark:text-amber-200">{{ currentWarningMessage }}</p>
                          </div>
                        </div>
                      </div>
                    </div>
                  </template>

                  <div v-if="conversationTurns.length === 0 && conversationHasPrompt" class="flex justify-end">
                    <div class="max-w-[min(42rem,88%)] rounded-lg rounded-tr-sm bg-primary-600 px-4 py-3 text-white shadow-sm">
                      <div class="mb-1 text-xs font-medium text-primary-100">{{ t('imageGeneration.userLabel') }}</div>
                      <p class="whitespace-pre-wrap break-words text-sm leading-6">{{ submittedPrompt }}</p>
                      <div class="mt-2 flex flex-wrap gap-2 text-xs text-primary-100">
                        <span>{{ submittedModel }}</span>
                        <span>{{ submittedSize }}</span>
                        <span>{{ submittedQuality }}</span>
                      </div>
                    </div>
                  </div>

                  <div v-if="conversationTurns.length === 0" class="flex justify-start">
                    <div class="max-w-[min(48rem,96%)] rounded-lg rounded-tl-sm border border-gray-100 bg-white px-4 py-3 shadow-sm dark:border-dark-700 dark:bg-dark-800">
                      <div class="mb-3 text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('imageGeneration.assistantLabel') }}</div>

                      <div v-if="currentWarningMessage && !generating" class="flex gap-3 rounded-lg border border-amber-100 bg-amber-50 p-3 dark:border-amber-900/60 dark:bg-amber-950/20">
                        <div class="flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-lg bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300">
                          <Icon name="infoCircle" size="sm" />
                        </div>
                        <div class="min-w-0">
                          <h2 class="text-sm font-semibold text-amber-900 dark:text-amber-100">{{ t('imageGeneration.resultUncertainTitle') }}</h2>
                          <p class="mt-1 break-words text-sm leading-6 text-amber-800 dark:text-amber-200">{{ currentWarningMessage }}</p>
                        </div>
                      </div>

                      <div v-else-if="currentFailureMessage && !generating" class="flex gap-3 rounded-lg border border-red-100 bg-red-50 p-3 dark:border-red-900/60 dark:bg-red-950/20">
                        <div class="flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-lg bg-red-100 text-red-600 dark:bg-red-900/40 dark:text-red-300">
                          <Icon name="x" size="sm" />
                        </div>
                        <div class="min-w-0">
                          <h2 class="text-sm font-semibold text-red-900 dark:text-red-100">{{ t('imageGeneration.failedTitle') }}</h2>
                          <p class="mt-1 break-words text-sm leading-6 text-red-700 dark:text-red-200">{{ currentFailureMessage }}</p>
                        </div>
                      </div>

                      <div v-else-if="generating" class="flex min-h-40 items-center gap-3 text-sm text-gray-500 dark:text-gray-400">
                        <LoadingSpinner size="md" />
                        <span>{{ t('imageGeneration.generating') }}</span>
                      </div>

                      <div v-else-if="selectedImageEntry">
                        <div class="overflow-hidden rounded-lg border border-gray-100 bg-gray-100 dark:border-dark-700 dark:bg-dark-900">
                          <img v-if="selectedImageEntry.src" :src="selectedImageEntry.src" :alt="selectedImageEntry.prompt" class="max-h-[560px] w-full object-contain" />
                          <div v-else-if="selectedImageEntry.loading" class="flex min-h-80 items-center justify-center">
                            <LoadingSpinner size="md" />
                          </div>
                          <div v-else class="flex min-h-80 flex-col items-center justify-center gap-2 px-4 text-center text-sm text-gray-500 dark:text-gray-400">
                            <Icon name="x" size="sm" />
                            <span>{{ t('imageGeneration.imageLoadFailed') }}</span>
                          </div>
                        </div>
                        <div class="mt-3 flex items-center justify-between gap-3">
                          <span class="min-w-0 truncate text-sm text-gray-500 dark:text-gray-400">
                            {{ selectedImageEntry.prompt }}
                          </span>
                          <div class="flex flex-shrink-0 items-center gap-1">
                            <button type="button" class="rounded-md p-1.5 text-gray-500 hover:bg-gray-100 hover:text-gray-700 disabled:cursor-not-allowed disabled:opacity-50 dark:hover:bg-dark-700 dark:hover:text-gray-200" :disabled="selectedImageEntry.loading || selectedImageEntry.failed" :title="t('imageGeneration.useAsReference')" @click="addReferenceImage(selectedImageEntry)">
                              <Icon name="link" size="sm" />
                            </button>
                            <button type="button" class="rounded-md p-1.5 text-gray-500 hover:bg-gray-100 hover:text-gray-700 dark:hover:bg-dark-700 dark:hover:text-gray-200" :title="t('imageGeneration.copyImage')" @click="copyImage(selectedImageEntry.image, selectedImageEntry.index, selectedImageEntry.historyId)">
                              <Icon name="copy" size="sm" />
                            </button>
                            <button type="button" class="rounded-md p-1.5 text-gray-500 hover:bg-gray-100 hover:text-gray-700 dark:hover:bg-dark-700 dark:hover:text-gray-200" :title="t('imageGeneration.download')" @click="downloadImage(selectedImageEntry.image, selectedImageEntry.index, selectedImageEntry.historyId)">
                              <Icon name="download" size="sm" />
                            </button>
                          </div>
                        </div>
                      </div>

                      <div v-else class="flex min-h-48 flex-col items-center justify-center px-3 py-5 text-center md:h-full md:min-h-0">
                        <div class="flex h-10 w-10 items-center justify-center rounded-lg bg-primary-50 text-primary-600 dark:bg-primary-900/30 dark:text-primary-300">
                          <Icon name="sparkles" size="md" />
                        </div>
                        <h2 class="mt-3 text-base font-semibold text-gray-900 dark:text-white">{{ t('imageGeneration.welcomeTitle') }}</h2>
                        <p class="mt-1 max-w-xl text-sm text-gray-500 dark:text-gray-400">{{ t('imageGeneration.welcomeSubtitle') }}</p>
                        <div class="mt-3 flex flex-wrap justify-center gap-2">
                          <button
                            v-for="sample in promptSamples"
                            :key="sample"
                            type="button"
                            class="rounded-full border border-gray-200 px-2.5 py-1 text-xs text-gray-700 transition-colors hover:border-primary-300 hover:text-primary-600 dark:border-dark-600 dark:text-gray-300 dark:hover:border-primary-600 dark:hover:text-primary-300"
                            @click="prompt = sample"
                          >
                            {{ sample }}
                          </button>
                        </div>
                      </div>
                    </div>
                  </div>
                </div>
              </div>
            </div>

            <aside class="w-full flex-shrink-0 space-y-3 md:h-full md:min-h-0 md:w-56 md:overflow-y-auto md:pr-1 xl:w-72">
              <div class="rounded-lg border border-gray-100 p-3 dark:border-dark-700">
                <label class="input-label">{{ t('imageGeneration.apiKey') }}</label>
                <select v-model="selectedApiKeyId" class="input mt-1">
                  <option v-for="option in imageGenerationKeyOptions" :key="option.id" :value="String(option.id)">
                    {{ apiKeyOptionLabel(option) }}
                  </option>
                </select>
              </div>

              <div class="rounded-lg border border-gray-100 p-3 dark:border-dark-700">
                <div class="mb-2 flex items-center justify-between">
                  <label class="input-label">{{ t('imageGeneration.model') }}</label>
                  <span class="text-xs text-gray-500 dark:text-gray-400">{{ costHint }}</span>
                </div>
                <select v-model="model" class="input">
                  <option v-for="option in modelOptions" :key="option" :value="option">{{ option }}</option>
                </select>
              </div>

              <ControlGroup :label="t('imageGeneration.aspectRatio')" :options="ratioOptions" v-model="size" />
              <ControlGroup :label="t('imageGeneration.quality')" :options="qualityOptions" v-model="quality" />
              <ControlGroup :label="t('imageGeneration.outputFormat')" :options="formatOptions" v-model="outputFormat" />
              <ControlGroup :label="t('imageGeneration.count')" :options="countOptions" v-model="countValue" />

              <div class="rounded-lg border border-gray-100 p-3 dark:border-dark-700">
                <label class="flex items-center justify-between gap-3 text-sm text-gray-700 dark:text-gray-200">
                  <span>{{ t('imageGeneration.transparentBackground') }}</span>
                  <input v-model="transparentBackground" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500" />
                </label>
                <label class="mt-3 block text-sm text-gray-700 dark:text-gray-200">
                  {{ t('imageGeneration.compression') }}
                  <input v-model.number="outputCompression" type="range" min="0" max="100" class="mt-2 w-full" />
                  <span class="text-xs text-gray-500 dark:text-gray-400">{{ outputCompression }}</span>
                </label>
              </div>
            </aside>
          </div>
        </section>

        <section class="flex-none rounded-lg border border-gray-100 bg-white p-3 dark:border-dark-700 dark:bg-dark-800 md:p-4">
          <div v-if="referenceImageEntries.length > 0" data-testid="reference-image-picker" class="mb-3 rounded-lg border border-gray-200 bg-gray-50 p-3 dark:border-dark-600 dark:bg-dark-900/40">
            <div class="mb-2 flex items-center justify-between gap-3">
              <div class="flex min-w-0 items-center gap-2 text-sm font-medium text-gray-700 dark:text-gray-200">
                <Icon name="link" size="sm" class="flex-shrink-0" />
                <span class="truncate">{{ t('imageGeneration.referenceImages') }}</span>
                <span class="flex-shrink-0 rounded-full bg-primary-50 px-2 py-0.5 text-xs text-primary-700 dark:bg-primary-900/30 dark:text-primary-300">{{ referenceImageEntries.length }}</span>
              </div>
              <button type="button" class="rounded-md px-2 py-1 text-xs font-medium text-gray-500 hover:bg-gray-100 hover:text-gray-700 dark:hover:bg-dark-700 dark:hover:text-gray-200" @click="clearReferenceImages">
                {{ t('imageGeneration.clearReferences') }}
              </button>
            </div>
            <div class="flex gap-2 overflow-x-auto pb-1">
              <div
                v-for="entry in referenceImageEntries"
                :key="entry.key"
                class="relative h-16 w-16 flex-shrink-0 overflow-hidden rounded-lg border border-gray-200 bg-white dark:border-dark-600 dark:bg-dark-800"
              >
                <img :src="entry.dataUrl" :alt="entry.prompt" class="h-full w-full object-cover" />
                <button type="button" class="absolute right-1 top-1 rounded-full bg-white/90 p-0.5 text-gray-600 shadow-sm hover:bg-white hover:text-gray-900 dark:bg-dark-800/90 dark:text-gray-300 dark:hover:text-white" :title="t('imageGeneration.removeReference')" @click="removeReferenceImage(entry.index)">
                  <Icon name="x" size="xs" />
                </button>
              </div>
            </div>
          </div>
          <textarea
            v-model="prompt"
            rows="3"
            class="input min-h-20 max-h-32 resize-y"
            :placeholder="t('imageGeneration.promptPlaceholder')"
            @keydown.enter="handlePromptEnter"
          ></textarea>
          <input
            ref="referenceFileInput"
            type="file"
            accept="image/*"
            multiple
            class="sr-only"
            @change="handleReferenceFileChange"
          />
          <div class="mt-3 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
            <div class="flex flex-wrap items-center gap-2">
              <button type="button" class="btn btn-secondary" @click="optimizePrompt">
                <Icon name="sparkles" size="sm" class="mr-2" />
                {{ t('imageGeneration.optimizePrompt') }}
              </button>
            </div>
            <div class="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-end">
              <button type="button" class="btn btn-secondary justify-center" :disabled="generating" @click="openReferenceImagePicker">
                <Icon name="upload" size="sm" class="mr-2" />
                {{ t('imageGeneration.addImage') }}
              </button>
              <button type="button" class="btn btn-primary justify-center" :disabled="!canGenerate || generating" @click="generateImage">
                <LoadingSpinner v-if="generating" size="sm" color="white" class="mr-2" />
                <Icon v-else name="arrowUp" size="sm" class="mr-2" />
                {{ t('imageGeneration.generate') }}
              </button>
            </div>
          </div>
        </section>
      </main>
    </div>

    <ConfirmDialog
      :show="conversationPendingDelete !== null"
      :title="t('imageGeneration.deleteConversation')"
      :message="t('imageGeneration.deleteConversationConfirm', { title: conversationPendingDelete?.title || '' })"
      :confirm-text="t('common.delete')"
      :cancel-text="t('common.cancel')"
      :danger="true"
      @confirm="confirmDeleteConversation"
      @cancel="cancelDeleteConversation"
    />
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, defineComponent, h, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { saveAs } from 'file-saver'
import AppLayout from '@/components/layout/AppLayout.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import keysAPI from '@/api/keys'
import type { ApiKey } from '@/types'
import imageGenerationAPI, {
  extractOpenAIImageError,
  isOpenAIImageRequestTimeout,
  resolveImageGenerationRequestSize,
  type ImageAspectRatio,
  type ImageGenerationHistoryRecord,
  type ImageGenerationPayload,
  type ImageOutputFormat,
  type ImageQuality,
  type ImageStyle,
  type OpenAIImageData,
} from '@/api/imageGeneration'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'

const ControlGroup = defineComponent({
  props: {
    label: { type: String, required: true },
    options: { type: Array as () => Array<{ label: string; value: string }>, required: true },
    modelValue: { type: String, required: true },
  },
  emits: ['update:modelValue'],
  setup(props, { emit }) {
    return () =>
      h('div', { class: 'rounded-lg border border-gray-100 p-3 dark:border-dark-700' }, [
        h('div', { class: 'input-label mb-2' }, props.label),
        h('div', { class: 'flex flex-wrap gap-2' }, props.options.map((option) =>
          h('button', {
            type: 'button',
            class: [
              'rounded-lg px-2.5 py-1 text-sm font-medium transition-colors',
              props.modelValue === option.value
                ? 'bg-primary-600 text-white shadow-sm'
                : 'bg-gray-100 text-gray-700 hover:bg-gray-200 dark:bg-dark-700 dark:text-gray-200 dark:hover:bg-dark-600',
            ],
            onClick: () => emit('update:modelValue', option.value),
          }, option.label),
        )),
      ])
  },
})

const { t } = useI18n()
const appStore = useAppStore()

const MAX_REFERENCE_IMAGE_BYTES = 20 * 1024 * 1024

interface ImageGenerationConversationItem {
  conversationId: number
  title: string
  latest: ImageGenerationHistoryRecord
  turns: ImageGenerationHistoryRecord[]
  turnCount: number
  updatedAt: string
}

interface ImageEntry {
  image: OpenAIImageData
  index: number
  historyId: number | null
  key: string
  src: string
  loading: boolean
  failed: boolean
  prompt: string
}

interface ImageSource {
  image: OpenAIImageData
  index: number
  historyId: number | null
  prompt: string
}

interface ReferenceImageEntry {
  dataUrl: string
  prompt: string
  sourceKey: string
}

interface ImageGenerationSubmission {
  payload: ImageGenerationPayload
  referenceImages: ReferenceImageEntry[]
  displaySize: string
}

const promptSamples = computed(() => [
  t('imageGeneration.samples.cat'),
  t('imageGeneration.samples.city'),
  t('imageGeneration.samples.garden'),
])

const apiKeys = ref<ApiKey[]>([])
const historyItems = ref<ImageGenerationHistoryRecord[]>([])
const historyLoading = ref(false)
const generating = ref(false)
const conversationPendingDelete = ref<ImageGenerationConversationItem | null>(null)
const conversationDeletePending = ref(false)
const prompt = ref('')
const submittedPrompt = ref('')
const submittedModel = ref('')
const submittedSize = ref('')
const submittedQuality = ref<ImageQuality | ''>('')
const currentResultPrompt = ref('')
const model = ref('gpt-image-2')
const style = ref<ImageStyle>('vivid')
const size = ref<ImageAspectRatio>('1:1')
const quality = ref<ImageQuality>('high')
const outputFormat = ref<ImageOutputFormat>('webp')
const countValue = ref('1')
const transparentBackground = ref(false)
const outputCompression = ref(80)
const currentImages = ref<OpenAIImageData[]>([])
const currentFailureMessage = ref('')
const currentWarningMessage = ref('')
const currentConversationId = ref<number | null>(null)
const conversationTurns = ref<ImageGenerationHistoryRecord[]>([])
const currentHistoryId = ref<number | null>(null)
const referenceImages = ref<ReferenceImageEntry[]>([])
const selectedImageIndex = ref(0)
const selectedApiKeyId = ref('')
const imagePreviewUrls = ref<Record<string, string>>({})
const imagePreviewBlobs = ref<Record<string, Blob>>({})
const imageClipboardBlobs = ref<Record<string, Blob>>({})
const imageClipboardDataURLs = ref<Record<string, string>>({})
const imagePreviewErrors = ref<Record<string, boolean>>({})
const referenceFileInput = ref<HTMLInputElement | null>(null)
const managedImagePreviewUrls = new Set<string>()
let imagePreviewRefreshVersion = 0
let conversationSelectionVersion = 0

const imageGenerationKeyOptions = computed(() => apiKeys.value)
const selectedApiKey = computed(() =>
  imageGenerationKeyOptions.value.find((key) => String(key.id) === selectedApiKeyId.value) || null,
)
const selectedApiKeyValue = computed(() => selectedApiKey.value?.key || '')
const canGenerate = computed(() => prompt.value.trim() !== '' && selectedApiKeyValue.value !== '')
const modelOptions = computed(() => ['gpt-image-2', 'gpt-image-1.5', 'gpt-image-1'])
const costHint = computed(() => t('imageGeneration.costHint', { count: countValue.value }))
const conversationHasPrompt = computed(() => submittedPrompt.value.trim() !== '')
const referenceImageEntries = computed(() =>
  referenceImages.value.map((entry, index) => ({
    ...entry,
    index,
    key: `${entry.sourceKey}:${index}`,
  })),
)
const historyConversations = computed<ImageGenerationConversationItem[]>(() => {
  const conversations = new Map<number, ImageGenerationConversationItem>()
  for (const item of historyItems.value) {
    const conversationId = recordConversationId(item)
    const existing = conversations.get(conversationId)
    if (!existing) {
      conversations.set(conversationId, {
        conversationId,
        title: item.conversation_title || item.prompt,
        latest: item,
        turns: [item],
        turnCount: 1,
        updatedAt: item.created_at,
      })
      continue
    }

    existing.turns.push(item)
    existing.turnCount += 1
    if (new Date(item.created_at).getTime() > new Date(existing.updatedAt).getTime()) {
      existing.latest = item
      existing.updatedAt = item.created_at
    }
  }
  return Array.from(conversations.values())
    .map((item) => ({
      ...item,
      turns: sortConversationTurns(item.turns),
    }))
    .sort((a, b) => new Date(b.updatedAt).getTime() - new Date(a.updatedAt).getTime())
})
const currentImageSources = computed<ImageSource[]>(() => {
  if (conversationTurns.value.length > 0) {
    return conversationTurns.value.flatMap((turn) => (turn.images || []).map((image, index) => ({
      image,
      index,
      historyId: turn.id,
      prompt: image.revised_prompt || turn.prompt,
    })))
  }

  return currentImages.value.map((image, index) => ({
    image,
    index,
    historyId: currentHistoryId.value,
    prompt: image.revised_prompt || currentResultPrompt.value || submittedPrompt.value,
  }))
})
const currentImageEntries = computed<ImageEntry[]>(() =>
  currentImageSources.value.map(({ image, index, historyId, prompt: imagePrompt }) => {
    const key = imagePreviewKey(image, index, historyId)
    const hasPreviewResult = Object.prototype.hasOwnProperty.call(imagePreviewUrls.value, key)
    return {
      image,
      index,
      historyId,
      key,
      src: imagePreviewUrls.value[key] || '',
      loading: !hasPreviewResult,
      failed: imagePreviewErrors.value[key] === true,
      prompt: imagePrompt,
    }
  }),
)
const selectedImageEntry = computed(() => currentImageEntries.value[selectedImageIndex.value] || currentImageEntries.value[0] || null)

function turnImageEntries(turn: ImageGenerationHistoryRecord): ImageEntry[] {
  return (turn.images || []).map((image, index) => {
    const key = imagePreviewKey(image, index, turn.id)
    const hasPreviewResult = Object.prototype.hasOwnProperty.call(imagePreviewUrls.value, key)
    return {
      image,
      index,
      historyId: turn.id,
      key,
      src: imagePreviewUrls.value[key] || '',
      loading: !hasPreviewResult,
      failed: imagePreviewErrors.value[key] === true,
      prompt: image.revised_prompt || turn.prompt,
    }
  })
}

const ratioValues: ImageAspectRatio[] = ['1:1', '16:9', '9:16', '4:3', '3:4']
const ratioOptions = ratioValues.map((value) => ({ label: value, value }))
const qualityOptions = [
  { label: '1K', value: 'low' },
  { label: '2K', value: 'medium' },
  { label: '4K', value: 'high' },
]
const formatOptions = [
  { label: 'WebP', value: 'webp' },
  { label: 'PNG', value: 'png' },
  { label: 'JPEG', value: 'jpeg' },
]
const countOptions = [
  { label: '1', value: '1' },
  { label: '2', value: '2' },
  { label: '4', value: '4' },
]
watch(imageGenerationKeyOptions, (keys) => {
  if (keys.length === 0) {
    selectedApiKeyId.value = ''
    return
  }
  if (!keys.some((key) => String(key.id) === selectedApiKeyId.value)) {
    selectedApiKeyId.value = String(keys[0].id)
  }
}, { immediate: true })

async function loadApiKeys() {
  try {
    const result = await keysAPI.list(1, 100, { status: 'active', image_generation_enabled: true })
    apiKeys.value = result.items
    if (imageGenerationKeyOptions.value.length === 0) {
      appStore.showError(t('imageGeneration.defaultApiKeyMissing'))
    }
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('imageGeneration.loadKeysFailed')))
  }
}

async function loadHistory() {
  historyLoading.value = true
  try {
    const result = await imageGenerationAPI.listHistory({
      page: 1,
      page_size: 50,
    })
    historyItems.value = result.items
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('imageGeneration.loadHistoryFailed')))
  } finally {
    historyLoading.value = false
  }
}

function apiKeyOptionLabel(apiKey: ApiKey): string {
  const groupName = apiKey.group?.name || t('imageGeneration.unknownGroup')
  return `${apiKey.name} · ${groupName}`
}

function recordConversationId(item: ImageGenerationHistoryRecord): number {
  return item.conversation_id || item.id
}

function sortConversationTurns(items: ImageGenerationHistoryRecord[]): ImageGenerationHistoryRecord[] {
  return [...items].sort((a, b) => {
    const turnDelta = (a.turn_index || 0) - (b.turn_index || 0)
    if (turnDelta !== 0) return turnDelta
    return new Date(a.created_at).getTime() - new Date(b.created_at).getTime()
  })
}

function isTimeoutUncertainMessage(message?: string | null): boolean {
  const normalized = (message || '').toLowerCase()
  return normalized.includes('timeout of') || (normalized.includes('timeout') && normalized.includes('exceeded'))
}

function isUncertainHistoryRecord(item: ImageGenerationHistoryRecord): boolean {
  return item.status === 'failed' && isTimeoutUncertainMessage(item.error_message)
}

function resetGenerationResult(clearPrompt = false) {
  if (clearPrompt) {
    prompt.value = ''
  }
  submittedPrompt.value = ''
  submittedModel.value = ''
  submittedSize.value = ''
  submittedQuality.value = ''
  currentResultPrompt.value = ''
  currentImages.value = []
  currentFailureMessage.value = ''
  currentWarningMessage.value = ''
  currentHistoryId.value = null
  referenceImages.value = []
  selectedImageIndex.value = 0
  clearImagePreviewUrls()
}

function startNewConversation() {
  conversationSelectionVersion += 1
  resetGenerationResult(true)
  currentConversationId.value = null
  conversationTurns.value = []
}

async function selectConversation(item: ImageGenerationConversationItem) {
  const selectionVersion = ++conversationSelectionVersion
  clearImagePreviewUrls()
  currentConversationId.value = item.conversationId
  conversationTurns.value = sortConversationTurns(item.turns)
  applyHistoryRecord(item.latest)

  try {
    const result = await imageGenerationAPI.listHistory({
      page: 1,
      page_size: 100,
      conversation_id: item.conversationId,
    })
    if (selectionVersion !== conversationSelectionVersion || currentConversationId.value !== item.conversationId) return
    conversationTurns.value = sortConversationTurns(result.items)
    const latest = conversationTurns.value[conversationTurns.value.length - 1] || item.latest
    applyHistoryRecord(latest)
  } catch (err: unknown) {
    if (selectionVersion !== conversationSelectionVersion) return
    appStore.showError(extractApiErrorMessage(err, t('imageGeneration.loadHistoryFailed')))
  }
}

function requestDeleteConversation(item: ImageGenerationConversationItem) {
  conversationPendingDelete.value = item
}

function cancelDeleteConversation() {
  if (!conversationDeletePending.value) {
    conversationPendingDelete.value = null
  }
}

async function confirmDeleteConversation() {
  const item = conversationPendingDelete.value
  if (!item || conversationDeletePending.value) return

  conversationDeletePending.value = true
  try {
    await imageGenerationAPI.deleteConversation(item.conversationId)
    historyItems.value = historyItems.value.filter((historyItem) =>
      recordConversationId(historyItem) !== item.conversationId,
    )
    if (currentConversationId.value === item.conversationId) {
      startNewConversation()
    }
    conversationPendingDelete.value = null
    appStore.showSuccess(t('imageGeneration.deleteConversationSuccess'))
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('imageGeneration.deleteConversationFailed')))
  } finally {
    conversationDeletePending.value = false
  }
}

function applyHistoryRecord(item: ImageGenerationHistoryRecord, syncPrompt = true) {
  currentHistoryId.value = item.id
  currentConversationId.value = recordConversationId(item)
  if (syncPrompt) {
    prompt.value = item.prompt
  }
  currentResultPrompt.value = item.prompt
  model.value = item.model
  size.value = normalizeAspectRatio(item.size)
  quality.value = normalizeQuality(item.quality)
  outputFormat.value = normalizeOutputFormat(item.output_format)
  countValue.value = String(Math.min(Math.max(item.n || 1, 1), 4))
  referenceImages.value = []
  currentImages.value = item.images || []
  currentFailureMessage.value = item.status === 'failed' && !isUncertainHistoryRecord(item)
    ? item.error_message || t('imageGeneration.generateFailed')
    : ''
  currentWarningMessage.value = isUncertainHistoryRecord(item) ? t('imageGeneration.generateTimeoutUncertain') : ''
  selectedImageIndex.value = 0
}

function appendConversationTurn(item: ImageGenerationHistoryRecord) {
  currentConversationId.value = recordConversationId(item)
  conversationTurns.value = sortConversationTurns([
    ...conversationTurns.value.filter((turn) => turn.id !== item.id),
    item,
  ])
  historyItems.value = [item, ...historyItems.value.filter((historyItem) => historyItem.id !== item.id)]
  applyHistoryRecord(item, false)
}

function normalizeQuality(value: string): ImageQuality {
  return value === 'low' || value === 'medium' || value === 'high' ? value : 'high'
}

function normalizeAspectRatio(value: string): ImageAspectRatio {
  if (ratioValues.includes(value as ImageAspectRatio)) {
    return value as ImageAspectRatio
  }
  const match = /^(\d+)x(\d+)$/i.exec(value.trim())
  if (!match) {
    return '1:1'
  }
  const width = Number(match[1])
  const height = Number(match[2])
  if (!Number.isFinite(width) || !Number.isFinite(height) || width <= 0 || height <= 0) {
    return '1:1'
  }
  const actual = width / height
  return ratioValues.reduce((best, candidate) => {
    const [candidateWidth, candidateHeight] = candidate.split(':').map(Number)
    const candidateDelta = Math.abs(actual - candidateWidth / candidateHeight)
    const bestDelta = Math.abs(actual - Number(best.split(':')[0]) / Number(best.split(':')[1]))
    return candidateDelta < bestDelta ? candidate : best
  }, '1:1' as ImageAspectRatio)
}

function normalizeOutputFormat(value: string): ImageOutputFormat {
  return value === 'png' || value === 'jpeg' || value === 'webp' ? value : 'webp'
}

function selectGeneratedImage(index: number) {
  selectedImageIndex.value = index
}

function turnReferenceImageEntries(turn: ImageGenerationHistoryRecord) {
  return turn.reference_images.map((dataUrl, index) => ({
    dataUrl,
    prompt: turn.prompt,
    key: `${turn.id}:${index}`,
  }))
}

async function addReferenceImage(entry: ImageEntry) {
  if (entry.loading || entry.failed) return
  if (referenceImages.value.some((image) => image.sourceKey === entry.key)) {
    appStore.showSuccess(t('imageGeneration.referenceImageAdded'))
    return
  }

  try {
    const dataUrl = await imageEntryToReferenceDataURL(entry)
    referenceImages.value = [
      ...referenceImages.value,
      {
        dataUrl,
        prompt: entry.prompt,
        sourceKey: entry.key,
      },
    ]
    appStore.showSuccess(t('imageGeneration.referenceImageAdded'))
  } catch {
    appStore.showError(t('imageGeneration.referenceImageFailed'))
  }
}

function openReferenceImagePicker() {
  referenceFileInput.value?.click()
}

async function handleReferenceFileChange(event: Event) {
  const input = event.target as HTMLInputElement
  const files = Array.from(input.files || [])
  input.value = ''

  if (files.length === 0) return

  const entries: ReferenceImageEntry[] = []
  for (const file of files) {
    if (!file.type.startsWith('image/')) {
      appStore.showError(t('imageGeneration.referenceImageInvalid'))
      continue
    }
    if (file.size > MAX_REFERENCE_IMAGE_BYTES) {
      appStore.showError(t('imageGeneration.referenceImageTooLarge'))
      continue
    }

    try {
      const dataUrl = await blobToDataURL(file)
      entries.push({
        dataUrl,
        prompt: file.name,
        sourceKey: `local-file:${file.name}:${file.size}:${file.lastModified}:${dataUrl.slice(0, 48)}`,
      })
    } catch {
      appStore.showError(t('imageGeneration.referenceImageFailed'))
    }
  }

  if (entries.length === 0) return

  const existingKeys = new Set(referenceImages.value.map((image) => image.sourceKey))
  const uniqueEntries = entries.filter((entry) => !existingKeys.has(entry.sourceKey))
  if (uniqueEntries.length === 0) {
    appStore.showSuccess(t('imageGeneration.referenceImageAdded'))
    return
  }

  referenceImages.value = [
    ...referenceImages.value,
    ...uniqueEntries,
  ]
  appStore.showSuccess(t('imageGeneration.referenceImageAdded'))
}

function removeReferenceImage(index: number) {
  referenceImages.value = referenceImages.value.filter((_, itemIndex) => itemIndex !== index)
}

function clearReferenceImages() {
  referenceImages.value = []
}

function handlePromptEnter(event: KeyboardEvent) {
  if (event.isComposing || event.shiftKey || event.ctrlKey || event.metaKey || event.altKey) return
  if (!canGenerate.value || generating.value) return

  event.preventDefault()
  void generateImage()
}

async function imageEntryToReferenceDataURL(entry: ImageEntry): Promise<string> {
  if (isImageDataURL(entry.image.url)) {
    return entry.image.url
  }

  const cachedBlob = imagePreviewBlobs.value[entry.key]
  if (cachedBlob) {
    return blobToDataURL(cachedBlob)
  }

  if (entry.image.b64_json) {
    return `data:${mimeTypeForOutputFormat(outputFormat.value)};base64,${imageDataURLBase64(entry.image.b64_json) || entry.image.b64_json}`
  }

  return blobToDataURL(await generatedImageToBlob(entry.image, entry.index, 'view', entry.historyId))
}

function base64ToUint8Array(value: string): Uint8Array {
  const base64 = value.includes(',') ? value.split(',').pop() || '' : value
  const binary = window.atob(base64)
  const bytes = new Uint8Array(binary.length)
  for (let i = 0; i < binary.length; i += 1) {
    bytes[i] = binary.charCodeAt(i)
  }
  return bytes
}

function isImageDataURL(value: string | undefined): value is string {
  return typeof value === 'string' && /^data:image\/[^;,]+;base64,/i.test(value.trim())
}

function imageDataURLBase64(value: string): string | null {
  if (!isImageDataURL(value)) return null
  const commaIndex = value.indexOf(',')
  if (commaIndex < 0) return null
  return value.slice(commaIndex + 1).trim() || null
}

function mimeTypeForDataURL(value: string): string {
  const match = value.match(/^data:([^;,]+)[;,]/i)
  return match?.[1] || mimeTypeForOutputFormat(outputFormat.value)
}

function imageDataURLToBlob(value: string): Blob {
  return new Blob([base64ToUint8Array(value)], { type: mimeTypeForDataURL(value) })
}

function mimeTypeForOutputFormat(format: ImageOutputFormat): string {
  return format === 'jpeg' ? 'image/jpeg' : `image/${format}`
}

function optimizePrompt() {
  const text = prompt.value.trim()
  const styleText = style.value === 'natural' ? t('imageGeneration.optimizeNatural') : t('imageGeneration.optimizeVivid')
  prompt.value = [text, styleText, t('imageGeneration.optimizeDetails')].filter(Boolean).join('，')
}

function buildPayload(promptText: string, activeReferenceImages: ReferenceImageEntry[] = referenceImages.value): ImageGenerationPayload {
  const payload: ImageGenerationPayload = {
    model: model.value,
    prompt: promptText,
    n: Number(countValue.value),
    size: resolveImageGenerationRequestSize(size.value, quality.value),
    quality: quality.value,
    response_format: 'url',
    style: style.value,
    background: transparentBackground.value ? 'transparent' : 'auto',
    output_format: transparentBackground.value ? 'png' : outputFormat.value,
    output_compression: outputFormat.value === 'webp' || outputFormat.value === 'jpeg' ? outputCompression.value : undefined,
    moderation: 'auto',
    reference_images: activeReferenceImages.length > 0 ? activeReferenceImages.map((image) => image.dataUrl) : undefined,
  }
  return payload
}

async function buildEditForm(payload: ImageGenerationPayload, activeReferenceImages: ReferenceImageEntry[]): Promise<FormData> {
  const form = new FormData()
  form.append('model', payload.model)
  form.append('prompt', payload.prompt)
  form.append('n', String(payload.n))
  form.append('size', payload.size)
  form.append('quality', payload.quality)
  form.append('response_format', payload.response_format)
  form.append('style', payload.style)
  form.append('background', payload.background)
  form.append('output_format', payload.output_format)
  form.append('moderation', payload.moderation)
  if (payload.output_compression !== undefined) {
    form.append('output_compression', String(payload.output_compression))
  }
  for (const [index, image] of activeReferenceImages.entries()) {
    const blob = await referenceImageToBlob(image)
    form.append(referenceImageFieldName(index, activeReferenceImages.length), blob, referenceImageFileName(blob, index, payload.output_format))
  }
  return form
}

function referenceImageFieldName(index: number, total: number): string {
  return total <= 1 && index === 0 ? 'image' : 'image[]'
}

function referenceImageFileName(blob: Blob, index: number, fallbackFormat: ImageOutputFormat): string {
  return `reference-${index + 1}.${fileExtensionForMimeType(blob.type || mimeTypeForOutputFormat(fallbackFormat))}`
}

async function referenceImageToBlob(image: ReferenceImageEntry): Promise<Blob> {
  if (isImageDataURL(image.dataUrl)) {
    return imageDataURLToBlob(image.dataUrl)
  }
  return fetchImageURLAsBlob(image.dataUrl)
}

async function submitImageGeneration(submission: ImageGenerationSubmission) {
  if (generating.value) return
  const apiKey = selectedApiKeyValue.value
  if (!apiKey) {
    appStore.showError(t('imageGeneration.defaultApiKeyMissing'))
    return
  }

  const { payload, referenceImages: activeReferenceImages, displaySize } = submission
  resetGenerationResult()
  submittedPrompt.value = payload.prompt
  submittedModel.value = payload.model
  submittedSize.value = displaySize
  submittedQuality.value = payload.quality
  currentResultPrompt.value = payload.prompt
  prompt.value = ''
  generating.value = true
  try {
    const response = activeReferenceImages.length > 0
      ? await imageGenerationAPI.edit(apiKey, await buildEditForm(payload, activeReferenceImages))
      : await imageGenerationAPI.generate(apiKey, payload)
    const historyImages = compactImageHistoryData(response.data || [])
    try {
      const saved = await imageGenerationAPI.saveHistory({
        api_key_id: selectedApiKey.value?.id ?? null,
        conversation_id: currentConversationId.value,
        prompt: payload.prompt,
        revised_prompt: response.data?.find((item) => item.revised_prompt)?.revised_prompt || null,
        model: payload.model,
        size: displaySize,
        quality: payload.quality,
        output_format: payload.output_format,
        n: payload.n,
        request: payload as unknown as Record<string, unknown>,
        reference_images: payload.reference_images || [],
        images: historyImages,
        status: 'succeeded',
      })
      appendConversationTurn(saved)
      clearReferenceImages()
      appStore.showSuccess(t('imageGeneration.generateSuccess'))
    } catch {
      appStore.showError(t('imageGeneration.saveHistoryFailed'))
      currentImages.value = []
      currentFailureMessage.value = t('imageGeneration.saveHistoryFailed')
      selectedImageIndex.value = 0
    }
  } catch (err: unknown) {
    const timedOut = isOpenAIImageRequestTimeout(err)
    const errorMessage = timedOut
      ? t('imageGeneration.generateTimeoutUncertain')
      : extractOpenAIImageError(err, t('imageGeneration.generateFailed'))
    currentImages.value = []
    currentFailureMessage.value = timedOut ? '' : errorMessage
    currentWarningMessage.value = timedOut ? errorMessage : ''
    selectedImageIndex.value = 0
    appStore.showError(errorMessage)
    if (timedOut) {
      return
    }
    try {
      const saved = await imageGenerationAPI.saveHistory({
        api_key_id: selectedApiKey.value?.id ?? null,
        conversation_id: currentConversationId.value,
        prompt: payload.prompt,
        revised_prompt: null,
        model: payload.model,
        size: displaySize,
        quality: payload.quality,
        output_format: payload.output_format,
        n: payload.n,
        request: payload as unknown as Record<string, unknown>,
        reference_images: payload.reference_images || [],
        images: [],
        status: 'failed',
        error_message: errorMessage,
      })
      appendConversationTurn(saved)
      clearReferenceImages()
    } catch {
      appStore.showError(t('imageGeneration.saveFailedTaskFailed'))
    }
  } finally {
    generating.value = false
  }
}

async function generateImage() {
  if (!canGenerate.value) return
  const promptText = prompt.value.trim()
  const activeReferenceImages = [...referenceImages.value]
  await submitImageGeneration({
    payload: buildPayload(promptText, activeReferenceImages),
    referenceImages: activeReferenceImages,
    displaySize: size.value,
  })
}

function isSavedImageGenerationPayload(value: unknown): value is ImageGenerationPayload {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return false
  const payload = value as Record<string, unknown>
  return typeof payload.model === 'string' && payload.model !== '' &&
    typeof payload.prompt === 'string' && payload.prompt !== '' &&
    typeof payload.n === 'number' && Number.isFinite(payload.n) && payload.n > 0 &&
    typeof payload.size === 'string' && payload.size !== '' &&
    (payload.quality === 'low' || payload.quality === 'medium' || payload.quality === 'high') &&
    (payload.response_format === 'url' || payload.response_format === 'b64_json') &&
    (payload.style === 'vivid' || payload.style === 'natural') &&
    (payload.background === 'auto' || payload.background === 'opaque' || payload.background === 'transparent') &&
    (payload.output_format === 'webp' || payload.output_format === 'png' || payload.output_format === 'jpeg') &&
    (payload.output_compression === undefined || (typeof payload.output_compression === 'number' && Number.isFinite(payload.output_compression))) &&
    (payload.moderation === 'auto' || payload.moderation === 'low') &&
    (payload.reference_images === undefined || (Array.isArray(payload.reference_images) && payload.reference_images.every((image) => typeof image === 'string')))
}

function retrySubmissionFromHistory(turn: ImageGenerationHistoryRecord): ImageGenerationSubmission | null {
  if (!isSavedImageGenerationPayload(turn.request) || !Array.isArray(turn.reference_images) || !turn.reference_images.every((image) => typeof image === 'string')) {
    return null
  }

  const referenceImages = turn.reference_images.map((dataUrl, index) => ({
    dataUrl,
    prompt: turn.prompt,
    sourceKey: `retry:${turn.id}:${index}`,
  }))
  return {
    payload: {
      ...turn.request,
      reference_images: referenceImages.length > 0 ? referenceImages.map((image) => image.dataUrl) : undefined,
    },
    referenceImages,
    displaySize: turn.size,
  }
}

async function retryFailedTurn(turn: ImageGenerationHistoryRecord) {
  if (generating.value || turn.status !== 'failed' || isUncertainHistoryRecord(turn)) return
  const submission = retrySubmissionFromHistory(turn)
  if (!submission) {
    appStore.showError(t('imageGeneration.retryUnavailable'))
    return
  }
  await submitImageGeneration(submission)
}

function imagePreviewKey(image: OpenAIImageData, index: number, historyId: number | null = currentHistoryId.value): string {
  return `${historyId ?? 'draft'}:${image.url || image.b64_json?.slice(0, 48) || index}`
}

async function copyImage(image: OpenAIImageData, index: number, historyId = currentHistoryId.value) {
  const key = imagePreviewKey(image, index, historyId)
  const cachedClipboardBlob = imageClipboardBlobs.value[key]
  if (cachedClipboardBlob && await copyPreparedImageToClipboard(cachedClipboardBlob, imageClipboardDataURLs.value[key], selectedImageEntry.value?.prompt || currentResultPrompt.value || submittedPrompt.value || prompt.value)) {
    appStore.showSuccess(t('imageGeneration.copied'))
    return
  }

  try {
    const blob = await generatedImageToBlob(image, index, 'view', historyId)
    const clipboardBlob = await clipboardCompatibleImageBlob(blob)
    if (clipboardBlob && await copyPreparedImageToClipboard(clipboardBlob, undefined, selectedImageEntry.value?.prompt || currentResultPrompt.value || submittedPrompt.value || prompt.value)) {
      appStore.showSuccess(t('imageGeneration.copied'))
      return
    }
  } catch {
    // Report the failure below; this action intentionally copies image data only.
  }

  appStore.showError(t('imageGeneration.copyFailed'))
}

async function copyPreparedImageToClipboard(blob: Blob, dataURL?: string, alt = ''): Promise<boolean> {
  if (await writeImageBlobToClipboard(blob)) return true

  try {
    return copyImageDataURLAsHTML(dataURL || await blobToDataURL(blob), alt)
  } catch {
    return false
  }
}

async function writeImageBlobToClipboard(blob: Blob): Promise<boolean> {
  if (!navigator.clipboard || !window.isSecureContext || typeof ClipboardItem === 'undefined') return false

  const mimeType = blob.type
  if (!mimeType.startsWith('image/')) return false
  if (typeof ClipboardItem.supports === 'function' && !ClipboardItem.supports(mimeType)) return false

  try {
    await navigator.clipboard.write([
      new ClipboardItem({ [mimeType]: blob }),
    ])
    return true
  } catch {
    return false
  }
}

function copyImageDataURLAsHTML(dataURL: string, alt: string): boolean {
  if (!dataURL.startsWith('data:image/')) return false
  const selection = window.getSelection()
  if (!selection) return false

  const container = document.createElement('div')
  container.contentEditable = 'true'
  container.style.cssText = 'position:fixed;left:-9999px;top:0;width:1px;height:1px;overflow:hidden;'

  const image = document.createElement('img')
  image.src = dataURL
  image.alt = alt
  container.appendChild(image)
  document.body.appendChild(container)

  try {
    container.focus()
    const range = document.createRange()
    range.selectNode(image)
    selection.removeAllRanges()
    selection.addRange(range)
    return document.execCommand('copy')
  } finally {
    selection.removeAllRanges()
    document.body.removeChild(container)
  }
}

function blobToDataURL(blob: Blob): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => {
      typeof reader.result === 'string' ? resolve(reader.result) : reject(new Error('Image conversion failed'))
    }
    reader.onerror = () => reject(reader.error || new Error('Image conversion failed'))
    reader.readAsDataURL(blob)
  })
}

async function clipboardCompatibleImageBlob(blob: Blob): Promise<Blob | null> {
  const mimeType = blob.type || mimeTypeForOutputFormat(outputFormat.value)
  if (mimeType === 'image/png') return blob.type ? blob : blob.slice(0, blob.size, mimeType)
  if (typeof ClipboardItem !== 'undefined' && typeof ClipboardItem.supports === 'function' && ClipboardItem.supports(mimeType)) {
    return blob.type ? blob : blob.slice(0, blob.size, mimeType)
  }
  if (!mimeType.startsWith('image/')) return null

  try {
    return await imageBlobToPngBlob(blob)
  } catch {
    return null
  }
}

async function imageBlobToPngBlob(blob: Blob): Promise<Blob> {
  const bitmap = await createImageBitmap(blob)
  try {
    const canvas = document.createElement('canvas')
    canvas.width = bitmap.width
    canvas.height = bitmap.height
    const context = canvas.getContext('2d')
    if (!context) throw new Error('Canvas 2D context is unavailable')
    context.drawImage(bitmap, 0, 0)
    const pngBlob = await new Promise<Blob | null>((resolve) => canvas.toBlob(resolve, 'image/png'))
    if (!pngBlob) throw new Error('PNG conversion failed')
    return pngBlob
  } finally {
    bitmap.close()
  }
}

async function fetchImageURLAsBlob(url: string): Promise<Blob> {
  const response = await fetch(url, { credentials: 'include' })
  if (!response.ok) throw new Error('Image fetch failed')
  const blob = await response.blob()
  if (!blob.type.startsWith('image/')) throw new Error('Fetched resource is not an image')
  return blob
}

async function downloadImage(image: OpenAIImageData, index: number, historyId = currentHistoryId.value) {
  const cachedBlob = imagePreviewBlobs.value[imagePreviewKey(image, index, historyId)]
  if (cachedBlob) {
    triggerImageDownload(cachedBlob, index)
    return
  }

  try {
    const blob = await generatedImageToBlob(image, index, 'download', historyId)
    triggerImageDownload(blob, index)
  } catch {
    appStore.showError(t('imageGeneration.downloadFailed'))
  }
}

function triggerImageDownload(blob: Blob, index: number) {
  saveAs(blob, `image-${Date.now()}-${index + 1}.${fileExtensionForMimeType(blob.type)}`)
}

async function generatedImageToBlob(
  image: OpenAIImageData,
  index: number,
  mode: 'view' | 'download' = 'download',
  historyId = currentHistoryId.value,
): Promise<Blob> {
  if (isImageDataURL(image.url)) {
    return imageDataURLToBlob(image.url)
  }

  const cachedPreviewBlob = imagePreviewBlobs.value[imagePreviewKey(image, index, historyId)]
  if (cachedPreviewBlob) {
    return cachedPreviewBlob
  }

  if (historyId !== null && image.url) {
    try {
      return await historyImageToBlob(historyId, index, mode)
    } catch (err) {
      if (!image.b64_json) throw err
    }
  }

  if (image.b64_json) {
    return new Blob([base64ToUint8Array(image.b64_json)], { type: mimeTypeForOutputFormat(outputFormat.value) })
  }

  if (image.url) {
    return fetchImageURLAsBlob(image.url)
  }

  if (historyId !== null) {
    return historyImageToBlob(historyId, index, mode)
  }

  throw new Error('Image proxy is unavailable')
}

async function historyImageToBlob(historyId: number, index: number, mode: 'view' | 'download'): Promise<Blob> {
  return mode === 'view'
    ? imageGenerationAPI.viewHistoryImage(historyId, index)
    : imageGenerationAPI.downloadHistoryImage(historyId, index)
}

function compactImageHistoryData(images: OpenAIImageData[]): OpenAIImageData[] {
  return images.map((image) => {
    const compacted: OpenAIImageData = {
      revised_prompt: image.revised_prompt ?? null,
    }
    const dataURLBase64 = image.url ? imageDataURLBase64(image.url) : null
    if (image.b64_json) {
      compacted.b64_json = image.b64_json
    } else if (dataURLBase64) {
      compacted.b64_json = dataURLBase64
    } else if (image.url) {
      compacted.url = image.url
    }
    return compacted
  })
}

async function refreshImagePreviewUrls() {
  clearImagePreviewUrls()
  const refreshVersion = imagePreviewRefreshVersion
  const entries: Record<string, string> = {}
  const blobs: Record<string, Blob> = {}
  const clipboardBlobs: Record<string, Blob> = {}
  const clipboardDataURLs: Record<string, string> = {}
  const errors: Record<string, boolean> = {}
  const refreshPreviewUrls = new Set<string>()

  await Promise.all(currentImageSources.value.map(async ({ image, index, historyId }) => {
    const key = imagePreviewKey(image, index, historyId)
    try {
      const blob = await generatedImageToBlob(image, index, 'view', historyId)
      const url = URL.createObjectURL(blob)
      refreshPreviewUrls.add(url)
      entries[key] = url
      blobs[key] = blob
      const clipboardBlob = await clipboardCompatibleImageBlob(blob)
      if (clipboardBlob) {
        clipboardBlobs[key] = clipboardBlob
        clipboardDataURLs[key] = await blobToDataURL(clipboardBlob)
      }
    } catch {
      entries[key] = ''
      errors[key] = true
    }
  }))

  if (refreshVersion !== imagePreviewRefreshVersion) {
    refreshPreviewUrls.forEach((url) => URL.revokeObjectURL(url))
    return
  }

  refreshPreviewUrls.forEach((url) => managedImagePreviewUrls.add(url))
  imagePreviewUrls.value = entries
  imagePreviewBlobs.value = blobs
  imageClipboardBlobs.value = clipboardBlobs
  imageClipboardDataURLs.value = clipboardDataURLs
  imagePreviewErrors.value = errors
}

function clearImagePreviewUrls() {
  imagePreviewRefreshVersion += 1
  managedImagePreviewUrls.forEach((url) => URL.revokeObjectURL(url))
  managedImagePreviewUrls.clear()
  imagePreviewUrls.value = {}
  imagePreviewBlobs.value = {}
  imageClipboardBlobs.value = {}
  imageClipboardDataURLs.value = {}
  imagePreviewErrors.value = {}
}

function fileExtensionForMimeType(mimeType: string): string {
  if (mimeType === 'image/jpeg') return 'jpg'
  if (mimeType.startsWith('image/')) return mimeType.slice('image/'.length) || outputFormat.value
  return outputFormat.value
}

function formatDate(value: string): string {
  if (!value) return ''
  return new Intl.DateTimeFormat(undefined, { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' }).format(new Date(value))
}

onMounted(() => {
  loadApiKeys()
  loadHistory()
})

watch([currentImages, conversationTurns, currentHistoryId, outputFormat], () => {
  void refreshImagePreviewUrls()
}, { deep: true })

watch(() => currentImageEntries.value.length, (imageCount) => {
  if (selectedImageIndex.value >= imageCount) {
    selectedImageIndex.value = 0
  }
})

onBeforeUnmount(() => {
  clearImagePreviewUrls()
})
</script>
