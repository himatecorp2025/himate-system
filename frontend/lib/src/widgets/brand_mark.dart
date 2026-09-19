import 'package:flutter/material.dart';

import '../theme/himate_theme.dart';

class HimateBrandMark extends StatelessWidget {
  const HimateBrandMark({
    this.compact = false,
    super.key,
  });

  final bool compact;

  @override
  Widget build(BuildContext context) {
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        SizedBox(
          width: compact ? 34 : 42,
          height: compact ? 34 : 42,
          child: const CustomPaint(painter: _BarMarkPainter()),
        ),
        if (!compact) ...[
          const SizedBox(width: 12),
          Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            mainAxisSize: MainAxisSize.min,
            children: [
              Text(
                'HIMATE',
                style: Theme.of(context).textTheme.titleLarge?.copyWith(
                      color: Colors.white,
                      fontWeight: FontWeight.w700,
                      letterSpacing: 2,
                    ),
              ),
              const Text(
                'SYSTEM',
                style: TextStyle(
                  color: Color(0xFFD5A23F),
                  fontSize: 11,
                  letterSpacing: 4,
                  fontWeight: FontWeight.w600,
                ),
              ),
            ],
          ),
        ],
      ],
    );
  }
}

class _BarMarkPainter extends CustomPainter {
  const _BarMarkPainter();

  @override
  void paint(Canvas canvas, Size size) {
    final paint = Paint()
      ..shader = const LinearGradient(
        begin: Alignment.bottomCenter,
        end: Alignment.topCenter,
        colors: [HimateColors.navySoft, HimateColors.navy, HimateColors.gold],
        stops: [0, 0.78, 1],
      ).createShader(Offset.zero & size);

    const count = 4;
    final gap = size.width * 0.07;
    final barWidth = (size.width - (count - 1) * gap) / count;
    for (var i = 0; i < count; i++) {
      final height = size.height * (0.45 + (i * 0.17));
      final left = i * (barWidth + gap);
      final top = size.height - height;
      final rect = RRect.fromRectAndRadius(
        Rect.fromLTWH(left, top, barWidth, height),
        Radius.circular(barWidth * 0.32),
      );
      canvas.drawRRect(rect, paint);
    }
  }

  @override
  bool shouldRepaint(covariant CustomPainter oldDelegate) => false;
}
