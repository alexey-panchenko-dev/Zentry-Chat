export type MediaFeature = 'voice' | 'profile_photos' | 'contact_profiles'
export function mediaCapability(config: unknown, feature: MediaFeature): boolean {
 if (!config || typeof config !== 'object' || !('media' in config)) return false
 const media = config.media
 return !!media && typeof media === 'object' && feature in media && (media as Record<string, unknown>)[feature] === true
}
