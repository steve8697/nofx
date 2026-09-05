import { useEffect, useRef } from 'react'
import * as THREE from 'three'

export type PageScene = 'competition' | 'traders' | 'trader'

interface Interactive3DBackgroundProps {
  currentPage?: PageScene
}

export function Interactive3DBackground({ currentPage = 'trader' }: Interactive3DBackgroundProps) {
  const containerRef = useRef<HTMLDivElement>(null)
  const sceneStateRef = useRef({
    currentPage,
    targetPage: currentPage,
  })

  useEffect(() => {
    sceneStateRef.current.targetPage = currentPage
  }, [currentPage])

  useEffect(() => {
    const container = containerRef.current
    if (!container) return

    // 1. Optimized Particle Grid Setup
    const SEPARATION = 55
    const AMOUNTX = 50
    const AMOUNTY = 50
    const numParticles = AMOUNTX * AMOUNTY

    const scene = new THREE.Scene()
    const camera = new THREE.PerspectiveCamera(
      55,
      window.innerWidth / window.innerHeight,
      1,
      10000
    )
    camera.position.set(0, 480, 1100)
    camera.lookAt(0, 50, 0)

    // 2. High-Efficiency WebGL Renderer
    const renderer = new THREE.WebGLRenderer({
      alpha: true,
      antialias: false,                   // GPU saving: AA unnecessary for point sprites
      powerPreference: 'low-power',       // GPU saving: favor integrated low-power silicon
    })
    renderer.setSize(window.innerWidth, window.innerHeight)
    // Standard 1.0x pixel ratio saves ~50% GPU fill-rate on Retina / 4K displays
    renderer.setPixelRatio(1.0)
    renderer.setClearColor(0x000000, 0)
    container.appendChild(renderer.domElement)

    // 3. Grid Coordinates & Radial Color Gradient Buffer
    const positions = new Float32Array(numParticles * 3)
    const colors = new Float32Array(numParticles * 3)
    const gridIndices = new Float32Array(numParticles * 2) // [ix, iy] for GPU vertex calculation

    const cJade = new THREE.Color(0x10B981)
    const cPlatinum = new THREE.Color(0xD1D5DB)

    let pIdx = 0
    let gIdx = 0
    for (let ix = 0; ix < AMOUNTX; ix++) {
      for (let iy = 0; iy < AMOUNTY; iy++) {
        positions[pIdx] = ix * SEPARATION - (AMOUNTX * SEPARATION) / 2
        positions[pIdx + 1] = 0.0 // Base height, computed on GPU
        positions[pIdx + 2] = iy * SEPARATION - (AMOUNTY * SEPARATION) / 2

        gridIndices[gIdx] = ix
        gridIndices[gIdx + 1] = iy

        const distNorm =
          Math.sqrt(
            Math.pow(ix - AMOUNTX / 2, 2) + Math.pow(iy - AMOUNTY / 2, 2)
          ) /
          (AMOUNTX * 0.5)

        const c =
          distNorm < 0.5
            ? cJade.clone().lerp(cPlatinum, distNorm * 2.0)
            : cPlatinum

        colors[pIdx] = c.r
        colors[pIdx + 1] = c.g
        colors[pIdx + 2] = c.b

        pIdx += 3
        gIdx += 2
      }
    }

    const geometry = new THREE.BufferGeometry()
    geometry.setAttribute('position', new THREE.BufferAttribute(positions, 3))
    geometry.setAttribute('color', new THREE.BufferAttribute(colors, 3))
    geometry.setAttribute('aGrid', new THREE.BufferAttribute(gridIndices, 2))

    // 4. Circular Particle Sprite Texture (High-clarity round particle glow)
    const canvas = document.createElement('canvas')
    canvas.width = 64
    canvas.height = 64
    const ctx = canvas.getContext('2d')
    if (ctx) {
      const grad = ctx.createRadialGradient(32, 32, 0, 32, 32, 32)
      grad.addColorStop(0, 'rgba(255, 255, 255, 1.0)')
      grad.addColorStop(0.35, 'rgba(255, 255, 255, 0.85)')
      grad.addColorStop(0.7, 'rgba(255, 255, 255, 0.25)')
      grad.addColorStop(1.0, 'rgba(255, 255, 255, 0.0)')
      ctx.fillStyle = grad
      ctx.fillRect(0, 0, 64, 64)
    }
    const texture = new THREE.CanvasTexture(canvas)

    // 5. Zero-CPU GPU Vertex Shader with Screen-Space Dynamic Liquid Ripples
    const customShaderMaterial = new THREE.ShaderMaterial({
      precision: 'mediump',
      uniforms: {
        uTime: { value: 0 },
        uPointTexture: { value: texture },
        uMouseNDC: { value: new THREE.Vector2(0, 0) },
        uAspect: { value: window.innerWidth / window.innerHeight },
        uMouseStrength: { value: 0.0 },
        uClickPosNDC: { value: new THREE.Vector2(-999, -999) },
        uClickTime: { value: -999.0 },
        uClickStrength: { value: 0.0 },
      },
      vertexShader: `
        precision mediump float;
        attribute vec2 aGrid;
        varying vec3 vColor;
        uniform float uTime;
        uniform vec2 uMouseNDC;
        uniform float uAspect;
        uniform float uMouseStrength;
        uniform vec2 uClickPosNDC;
        uniform float uClickTime;
        uniform float uClickStrength;

        void main() {
          vColor = color;
          
          // 1. Natural ambient dual sine waves
          float baseWave = sin((aGrid.x + uTime) * 0.3) * 65.0 + sin((aGrid.y + uTime) * 0.5) * 65.0;

          // 2. Project particle to Viewport Screen Space (NDC) for 100% accurate cursor tracking
          vec4 baseMv = modelViewMatrix * vec4(position.x, position.y + baseWave, position.z, 1.0);
          vec4 baseClip = projectionMatrix * baseMv;
          vec2 ndc = baseClip.xy / baseClip.w;

          // Correct for screen aspect ratio so wave ripples are circular in viewport
          vec2 ndcAspect = vec2(ndc.x * uAspect, ndc.y);
          vec2 mouseAspect = vec2(uMouseNDC.x * uAspect, uMouseNDC.y);
          float distScreen = distance(ndcAspect, mouseAspect);

          // 3. Continuous Fluid Bow Wave (Naturally generated as cursor moves)
          float wavePhase1 = distScreen * 14.0 - uTime * 3.2;
          float wavePhase2 = distScreen * 24.0 - uTime * 4.8;
          float harmonicRipple = (sin(wavePhase1) * 0.70 + sin(wavePhase2) * 0.30) * exp(-distScreen * 2.5) * 60.0 * uMouseStrength;

          // Smooth surface tension buoyancy lift:
          float lift = smoothstep(0.48, 0.0, distScreen) * 36.0 * uMouseStrength;

          // 4. Click Water Shockwave (Pure GPU continuous propagation - Zero DOM Jank)
          vec2 clickAspect = vec2(uClickPosNDC.x * uAspect, uClickPosNDC.y);
          float distClick = distance(ndcAspect, clickAspect);
          float timeSinceClick = max(0.0, uTime - uClickTime);
          float clickWave = 0.0;

          if (timeSinceClick < 3.0 && uClickStrength > 0.01) {
            float waveRadius = timeSinceClick * 0.75;
            float distFromFront = abs(distClick - waveRadius);
            clickWave = sin(distFromFront * 20.0) * smoothstep(0.35, 0.0, distFromFront) * exp(-distClick * 1.8) * exp(-timeSinceClick * 1.5) * 88.0 * uClickStrength;
            
            // Subtle emerald radiance along the shockwave crest
            if (distFromFront < 0.25) {
              float crestGlow = (1.0 - distFromFront / 0.25) * exp(-timeSinceClick * 1.5) * uClickStrength;
              vColor += vec3(0.12, 0.50, 0.38) * crestGlow;
            }
          }

          vec3 transformed = vec3(position.x, position.y + baseWave + harmonicRipple + lift + clickWave, position.z);

          // Subtle emerald luminescence when mouse touches particles (diffuse aurora)
          if (uMouseStrength > 0.01 && distScreen < 0.50) {
            float glowFactor = (1.0 - distScreen / 0.50) * uMouseStrength;
            vColor += vec3(0.08, 0.38, 0.28) * glowFactor;
          }

          vec4 mvPosition = modelViewMatrix * vec4(transformed, 1.0);
          gl_Position = projectionMatrix * mvPosition;

          // Point attenuation calibrated for clean, visible depth
          float sizeBoost = (distScreen < 0.32) ? (1.0 + (1.0 - distScreen / 0.32) * 0.5 * uMouseStrength) : 1.0;
          gl_PointSize = clamp((5200.0 / -mvPosition.z) * sizeBoost, 3.2, 13.0);
        }
      `,
      fragmentShader: `
        precision mediump float;
        varying vec3 vColor;
        uniform sampler2D uPointTexture;

        void main() {
          vec4 texColor = texture2D(uPointTexture, gl_PointCoord);
          if (texColor.a < 0.02) discard;
          gl_FragColor = vec4(vColor * 1.25, texColor.a * 0.95);
        }
      `,
      transparent: true,
      blending: THREE.AdditiveBlending,
      depthWrite: false,
      vertexColors: true,
    })

    const particles = new THREE.Points(geometry, customShaderMaterial)
    particles.position.set(0, 0, -100)
    scene.add(particles)

    // 6. User Pointer Tracking & Screen-Space Wave Raycasting
    let mouseX = 0
    let mouseY = 0
    let targetMouseX = 0
    let targetMouseY = 0

    const currentMouseNDC = new THREE.Vector2(0, 0)
    const targetMouseNDC = new THREE.Vector2(0, 0)

    let animationFrameId: number
    let isVisible = true
    let lastFrameTime = 0
    let lastInteractionTime = performance.now()

    const onVisibilityChange = () => {
      isVisible = !document.hidden
      if (isVisible) {
        lastFrameTime = performance.now()
        lastInteractionTime = performance.now()
        animationFrameId = requestAnimationFrame(animate)
      }
    }
    document.addEventListener('visibilitychange', onVisibilityChange)

    let isRunning = true
    let lastCardRippleTime = 0
    let lastCardElement: HTMLElement | null = null

    const triggerCardRipple = (card: HTMLElement, clientX: number, clientY: number, isStrong: boolean = false) => {
      const rect = card.getBoundingClientRect()
      const x = clientX - rect.left
      const y = clientY - rect.top

      const createRing = (className: string, delayMs: number = 0) => {
        setTimeout(() => {
          const ripple = document.createElement('div')
          ripple.className = className
          ripple.style.left = `${x}px`
          ripple.style.top = `${y}px`
          card.appendChild(ripple)
          setTimeout(() => {
            if (ripple.parentNode === card) {
              card.removeChild(ripple)
            }
          }, 1100)
        }, delayMs)
      }

      // Primary wave crest
      createRing(isStrong ? 'liquid-water-ripple liquid-water-ripple-strong' : 'liquid-water-ripple', 0)

      // Natural secondary trailing rebound ripple (干涉餘波, 80ms delay)
      if (isStrong) {
        createRing('liquid-water-ripple liquid-water-ripple-secondary', 80)
      }
    }

    const onPointerMove = (event: MouseEvent) => {
      const now = performance.now()
      lastInteractionTime = now
      targetMouseX = event.clientX - window.innerWidth / 2
      targetMouseY = event.clientY - window.innerHeight / 2

      // Viewport Normalized Coordinates [-1, 1]
      targetMouseNDC.x = (event.clientX / window.innerWidth) * 2 - 1
      targetMouseNDC.y = -(event.clientY / window.innerHeight) * 2 + 1

      // Gentle water ripple on entering or gliding across card panes (zero reflow, using event.target)
      const targetCard = (event.target as HTMLElement)?.closest?.(
        '.sharp-card, .glass-card, .binance-card, .modal-content, [role="dialog"], .stat-card-pane, .telemetry-strip'
      ) as HTMLElement | null
      if (targetCard) {
        const isNewCard = targetCard !== lastCardElement
        if (isNewCard || now - lastCardRippleTime > 450) {
          lastCardRippleTime = now
          lastCardElement = targetCard
          triggerCardRipple(targetCard, event.clientX, event.clientY, false)
        }
      } else {
        lastCardElement = null
      }

      if (!isRunning && isVisible) {
        isRunning = true
        lastFrameTime = now
        animationFrameId = requestAnimationFrame(animate)
      }
    }

    const onPointerDown = (event: MouseEvent) => {
      const now = performance.now()
      lastInteractionTime = now
      
      // Calculate Normalized Device Coordinates [-1, 1] for click
      const clickNDC = new THREE.Vector2(
        (event.clientX / window.innerWidth) * 2 - 1,
        -(event.clientY / window.innerHeight) * 2 + 1
      )
      
      customShaderMaterial.uniforms.uClickPosNDC.value.copy(clickNDC)
      customShaderMaterial.uniforms.uClickTime.value = timeAccumulator
      customShaderMaterial.uniforms.uClickStrength.value = 1.0

      // Trigger strong shockwave water ripple on clicked card
      const targetCard = (event.target as HTMLElement)?.closest?.(
        '.sharp-card, .glass-card, .binance-card, .modal-content, [role="dialog"], .stat-card-pane, .telemetry-strip'
      ) as HTMLElement | null
      if (targetCard) {
        lastCardRippleTime = now
        triggerCardRipple(targetCard, event.clientX, event.clientY, true)
      }
      
      if (!isRunning && isVisible) {
        isRunning = true
        lastFrameTime = now
        animationFrameId = requestAnimationFrame(animate)
      }
    }

    const onResize = () => {
      camera.aspect = window.innerWidth / window.innerHeight
      camera.updateProjectionMatrix()
      renderer.setSize(window.innerWidth, window.innerHeight)
      customShaderMaterial.uniforms.uAspect.value = window.innerWidth / window.innerHeight
    }

    window.addEventListener('pointermove', onPointerMove, { passive: true })
    window.addEventListener('pointerdown', onPointerDown, { passive: true })
    window.addEventListener('resize', onResize)

    // 7. Dynamic Progressive Power-Saver Loop (Active 60fps for butter-smooth fluidity)
    let timeAccumulator = 0

    const animate = (now: number = 0) => {
      if (!isVisible) {
        isRunning = false
        return
      }

      // Progressive Energy Tiers:
      const idleTime = now - lastInteractionTime
      if (idleTime > 5000 && customShaderMaterial.uniforms.uClickStrength.value <= 0.01) {
        isRunning = false
        return
      }

      // 60 FPS when interacting (16ms) for fluid motion, smoothly throttle down when idle
      const targetInterval = idleTime > 2500 ? 60 : 16
      const elapsed = now - lastFrameTime

      if (elapsed < targetInterval) {
        animationFrameId = requestAnimationFrame(animate)
        return
      }

      lastFrameTime = now - (elapsed % targetInterval)
      animationFrameId = requestAnimationFrame(animate)

      // Smooth camera interpolation
      mouseX += (targetMouseX - mouseX) * 0.05
      mouseY += (targetMouseY - mouseY) * 0.05

      const activePage = sceneStateRef.current.targetPage
      let targetCamY = 480
      let targetCamZ = 1100

      if (activePage === 'competition') {
        targetCamY = 600
        targetCamZ = 1250
      } else if (activePage === 'traders') {
        targetCamY = 420
        targetCamZ = 1000
      }

      camera.position.x += (mouseX * 0.8 - camera.position.x) * 0.05
      camera.position.y += (-mouseY * 0.6 + targetCamY - camera.position.y) * 0.05
      camera.position.z += (targetCamZ - camera.position.z) * 0.05
      camera.lookAt(0, 40, 0)

      // Fluid screen-space mouse tracking & wave decay
      currentMouseNDC.x += (targetMouseNDC.x - currentMouseNDC.x) * 0.16
      currentMouseNDC.y += (targetMouseNDC.y - currentMouseNDC.y) * 0.16
      const waveStrength = idleTime < 800 ? 1.0 : Math.max(0.0, 1.0 - (idleTime - 800) / 1400)

      customShaderMaterial.uniforms.uMouseNDC.value.copy(currentMouseNDC)
      customShaderMaterial.uniforms.uMouseStrength.value = waveStrength

      // Click shockwave smooth exponential decay
      if (customShaderMaterial.uniforms.uClickStrength.value > 0.005) {
        customShaderMaterial.uniforms.uClickStrength.value *= 0.965
      } else {
        customShaderMaterial.uniforms.uClickStrength.value = 0.0
      }

      // Zero-CPU Sine calculation: offloaded entirely to GPU shader!
      timeAccumulator += 0.06
      customShaderMaterial.uniforms.uTime.value = timeAccumulator

      renderer.render(scene, camera)
    }

    animate()

    return () => {
      isVisible = false
      cancelAnimationFrame(animationFrameId)
      document.removeEventListener('visibilitychange', onVisibilityChange)
      window.removeEventListener('pointermove', onPointerMove)
      window.removeEventListener('pointerdown', onPointerDown)
      window.removeEventListener('resize', onResize)
      if (container.contains(renderer.domElement)) {
        container.removeChild(renderer.domElement)
      }
      renderer.dispose()
      geometry.dispose()
      customShaderMaterial.dispose()
      texture.dispose()
    }
  }, [])

  return (
    <div
      ref={containerRef}
      className="fixed inset-0 pointer-events-none z-0 overflow-hidden"
      style={{
        opacity: 0.95,
        willChange: 'transform',
      }}
    />
  )
}
