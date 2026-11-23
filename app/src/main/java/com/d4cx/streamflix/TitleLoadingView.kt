package com.d4cx.streamflix

import android.content.Context
import android.graphics.Bitmap
import android.graphics.Canvas
import android.graphics.Color
import android.graphics.ColorMatrix
import android.graphics.ColorMatrixColorFilter
import android.graphics.Paint
import android.graphics.Path
import android.graphics.RectF
import android.graphics.Typeface
import android.os.SystemClock
import android.text.Layout
import android.text.StaticLayout
import android.text.TextPaint
import android.util.AttributeSet
import android.view.View
import kotlin.math.sin

/**
 * Stremio-style Liquid Glass-Fill Title / Official Logo Loader for Android TV.
 * - Starts completely grayed out inside the text or official transparent logo PNG.
 * - Fills smoothly bottom-to-top with a live animated liquid sine wave as the stream loads.
 * - Zero heavy shader overhead; crisp 30fps Canvas clipPath tuned for low-RAM Xiaomi Mi TV.
 */
class TitleLoadingView @JvmOverloads constructor(
    context: Context,
    attrs: AttributeSet? = null
) : View(context, attrs) {

    var text: CharSequence = ""
        set(value) {
            val clean = value.toString().trim().uppercase()
            if (field != clean) {
                field = clean
                cachedLayoutWidth = -1
                invalidate()
            }
        }

    private var logoBitmap: Bitmap? = null
    private var targetFill: Float = 0f
    private var displayFill: Float = 0f
    private var lastFrameMs: Long = 0L

    private val wavePath = Path()
    private val logoDest = RectF()

    private val dimLogoPaint = Paint(Paint.ANTI_ALIAS_FLAG or Paint.FILTER_BITMAP_FLAG).apply {
        val cm = ColorMatrix().apply { setSaturation(0f) }
        colorFilter = ColorMatrixColorFilter(cm)
        alpha = 64
    }

    private val fillLogoPaint = Paint(Paint.ANTI_ALIAS_FLAG or Paint.FILTER_BITMAP_FLAG).apply {
        alpha = 255
    }

    private val dimTextPaint = TextPaint(Paint.ANTI_ALIAS_FLAG).apply {
        color = Color.argb(66, 148, 163, 184)
        typeface = Typeface.create("sans-serif-black", Typeface.BOLD)
        letterSpacing = 0.12f
        textSize = spToPx(42f)
    }

    private val strokeTextPaint = TextPaint(Paint.ANTI_ALIAS_FLAG).apply {
        color = Color.argb(44, 255, 255, 255)
        style = Paint.Style.STROKE
        strokeWidth = dpToPx(1.1f)
        typeface = Typeface.create("sans-serif-black", Typeface.BOLD)
        letterSpacing = 0.12f
        textSize = spToPx(42f)
    }

    private val fillTextPaint = TextPaint(Paint.ANTI_ALIAS_FLAG).apply {
        color = Color.WHITE
        typeface = Typeface.create("sans-serif-black", Typeface.BOLD)
        letterSpacing = 0.12f
        textSize = spToPx(42f)
        setShadowLayer(dpToPx(12f), 0f, 0f, Color.argb(120, 255, 255, 255))
    }

    private var cachedLayoutWidth = -1
    private var dimLayout: StaticLayout? = null
    private var strokeLayout: StaticLayout? = null
    private var fillLayout: StaticLayout? = null

    init {
        setLayerType(LAYER_TYPE_SOFTWARE, null)
    }

    private fun dpToPx(dp: Float): Float = dp * resources.displayMetrics.density
    private fun spToPx(sp: Float): Float = sp * resources.displayMetrics.scaledDensity

    fun setLogoBitmap(bmp: Bitmap?) {
        if (bmp != null && !bmp.isRecycled) {
            logoBitmap = bmp
            invalidate()
        } else {
            logoBitmap = null
            invalidate()
        }
    }

    fun resetFill() {
        targetFill = 0f
        displayFill = 0f
        invalidate()
    }

    /** Advances liquid level monotonically (0f = grayed out empty, 1f = completely full). */
    fun setFill(value: Float) {
        if (value < 0f) return
        val clamped = value.coerceIn(0f, 1f)
        if (clamped > targetFill) {
            targetFill = clamped
            invalidate()
        }
    }

    fun forceFill(value: Float) {
        val clamped = value.coerceIn(0f, 1f)
        targetFill = clamped
        displayFill = clamped
        invalidate()
    }

    fun getTargetFill(): Float = targetFill

    private fun ensureTextLayouts(availWidth: Int) {
        if (availWidth <= 0) return
        val str = text.toString()
        if (cachedLayoutWidth == availWidth && dimLayout != null) return
        cachedLayoutWidth = availWidth

        val baseSize = when {
            str.length <= 12 -> spToPx(46f)
            str.length <= 22 -> spToPx(38f)
            str.length <= 36 -> spToPx(31f)
            else -> spToPx(25f)
        }
        dimTextPaint.textSize = baseSize
        strokeTextPaint.textSize = baseSize
        fillTextPaint.textSize = baseSize

        dimLayout = buildLayout(str, dimTextPaint, availWidth)
        strokeLayout = buildLayout(str, strokeTextPaint, availWidth)
        fillLayout = buildLayout(str, fillTextPaint, availWidth)
    }

    @Suppress("DEPRECATION")
    private fun buildLayout(str: String, paint: TextPaint, width: Int): StaticLayout {
        return StaticLayout.Builder.obtain(str, 0, str.length, paint, width)
            .setAlignment(Layout.Alignment.ALIGN_CENTER)
            .setLineSpacing(0f, 1.06f)
            .setIncludePad(false)
            .setMaxLines(2)
            .build()
    }

    private fun buildLiquidClipPath(left: Float, top: Float, right: Float, bottom: Float, fillFrac: Float, phaseRad: Double) {
        wavePath.reset()
        val boxHeight = (bottom - top).coerceAtLeast(1f)
        val boxWidth = (right - left).coerceAtLeast(1f)
        // Map 0 -> bottom + waveAmp (completely empty), 1 -> top - waveAmp (completely full)
        val maxAmp = (boxHeight * 0.085f).coerceIn(dpToPx(3f), dpToPx(10f))
        val amp = when {
            fillFrac <= 0.01f || fillFrac >= 0.99f -> 0f
            fillFrac < 0.1f -> maxAmp * (fillFrac / 0.1f)
            fillFrac > 0.9f -> maxAmp * ((1f - fillFrac) / 0.1f)
            else -> maxAmp
        }
        val waterY = bottom - (boxHeight + maxAmp * 1.4f) * fillFrac + maxAmp * 0.7f

        wavePath.moveTo(left - 4f, bottom + 12f)
        wavePath.lineTo(left - 4f, waterY)

        val steps = 28
        val dx = (boxWidth + 8f) / steps
        for (i in 0..steps) {
            val x = (left - 4f) + i * dx
            val normX = i.toDouble() / steps.toDouble()
            // Double harmonic wave for natural glass-fill liquid surface
            val wave = sin(normX * Math.PI * 3.0 + phaseRad) * amp +
                sin(normX * Math.PI * 5.2 - phaseRad * 1.35) * (amp * 0.35)
            wavePath.lineTo(x, (waterY + wave).toFloat())
        }

        wavePath.lineTo(right + 4f, bottom + 12f)
        wavePath.close()
    }

    override fun onDraw(canvas: Canvas) {
        super.onDraw(canvas)
        val w = width
        val h = height
        if (w <= 0 || h <= 0) return

        val now = SystemClock.uptimeMillis()
        val dt = if (lastFrameMs == 0L) 33L else (now - lastFrameMs).coerceIn(8L, 120L)
        lastFrameMs = now

        // Smooth organic interpolation toward targetFill
        val diff = targetFill - displayFill
        if (kotlin.math.abs(diff) > 0.002f) {
            val step = (dt / 1000f) * 1.85f
            displayFill += diff * step.coerceAtMost(0.45f)
            if (kotlin.math.abs(targetFill - displayFill) < 0.003f) {
                displayFill = targetFill
            }
        }

        val phaseRad = (now % 100000L) / 260.0

        val bmp = logoBitmap
        if (bmp != null && !bmp.isRecycled) {
            // Render Official Transparent Movie/Series Logo PNG with bottom-to-top liquid fill
            val padX = dpToPx(24f)
            val padY = dpToPx(12f)
            val maxW = (w - padX * 2).coerceAtLeast(1f)
            val maxH = (h - padY * 2).coerceAtLeast(1f)
            val scale = minOf(maxW / bmp.width.toFloat(), maxH / bmp.height.toFloat())
            val drawW = bmp.width * scale
            val drawH = bmp.height * scale
            val left = (w - drawW) * 0.5f
            val top = (h - drawH) * 0.5f
            logoDest.set(left, top, left + drawW, top + drawH)

            // 1. Grayed-out dim base logo
            canvas.drawBitmap(bmp, null, logoDest, dimLogoPaint)

            // 2. Rising liquid-wave full-luminance logo
            if (displayFill > 0.004f) {
                buildLiquidClipPath(logoDest.left, logoDest.top, logoDest.right, logoDest.bottom, displayFill, phaseRad)
                val saveCount = canvas.save()
                canvas.clipPath(wavePath)
                canvas.drawBitmap(bmp, null, logoDest, fillLogoPaint)
                canvas.restoreToCount(saveCount)
            }
        } else {
            // Render Crisp Monumental Typography with bottom-to-top liquid wave glass fill
            val padX = dpToPx(16f).toInt()
            val availW = (w - padX * 2).coerceAtLeast(1)
            ensureTextLayouts(availW)
            val dl = dimLayout ?: return
            val sl = strokeLayout ?: return
            val fl = fillLayout ?: return

            val textH = dl.height.toFloat()
            val top = ((h - textH) * 0.5f).coerceAtLeast(0f)

            // 1. Grayed-out base text + hairline glass edge stroke
            canvas.save()
            canvas.translate(padX.toFloat(), top)
            dl.draw(canvas)
            sl.draw(canvas)
            canvas.restore()

            // 2. Rising liquid-wave luminous white fill inside the exact letters
            if (displayFill > 0.004f) {
                buildLiquidClipPath(
                    padX.toFloat(),
                    top,
                    (w - padX).toFloat(),
                    top + textH,
                    displayFill,
                    phaseRad
                )
                val saveCount = canvas.save()
                canvas.clipPath(wavePath)
                canvas.translate(padX.toFloat(), top)
                fl.draw(canvas)
                canvas.restoreToCount(saveCount)
            }
        }

        if (isShown && visibility == VISIBLE) {
            postInvalidateDelayed(33L)
        }
    }
}
