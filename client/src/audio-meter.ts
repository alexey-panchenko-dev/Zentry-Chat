// Firefox does not consistently expose microphone audioLevel in getStats().
// Measure the captured samples locally, without routing the microphone to speakers.
import { acquireCallAudioContext } from './call-audio-context.ts'
export class AudioMeter {
  private context?: AudioContext
  private lease?: ReturnType<typeof acquireCallAudioContext>
  private source?: MediaStreamAudioSourceNode
  private analyser?: AnalyserNode
  private stream?: MediaStream
  unlock() {
    try {
      this.lease ??= acquireCallAudioContext()
      this.context = this.lease?.context
      if (!this.context) return
      if (this.context.state !== 'running') void this.context.resume().catch(() => {})
    } catch { /* Meter availability must not prevent a call. */ }
  }
  sample(stream: MediaStream): number | null {
    if (!this.context || this.context.state !== 'running') return null
    try {
      if (stream !== this.stream) {
        this.disconnect()
        this.stream = stream
        this.analyser = this.context.createAnalyser()
        this.analyser.fftSize = 1024
        this.source = this.context.createMediaStreamSource(new MediaStream(stream.getAudioTracks()))
        this.source.connect(this.analyser)
      }
      const values = new Float32Array(this.analyser!.fftSize)
      this.analyser!.getFloatTimeDomainData(values)
      return Math.min(1, Math.sqrt(values.reduce((sum,value) => sum+value*value,0)/values.length))
    } catch { return null }
  }
  disconnect() { this.source?.disconnect(); this.source=undefined; this.analyser=undefined; this.stream=undefined }
  dispose() { this.disconnect();this.lease?.release();this.lease=undefined;this.context=undefined }
}
