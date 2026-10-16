import { inject, type InjectionKey } from 'vue'
import type { PhotoManager } from './usePhotoManager'

export const photoManagerKey: InjectionKey<PhotoManager> = Symbol('photoManager')

// All routes share the manager created once by app.vue.
export function useSharedPhotoManager(): PhotoManager {
  const manager = inject(photoManagerKey)
  if (!manager) throw new Error('Photo manager must be provided by app.vue')
  return manager
}
