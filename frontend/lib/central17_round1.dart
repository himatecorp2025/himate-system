part of 'main.dart';

class _SidebarMountainArt extends StatelessWidget {
  const _SidebarMountainArt();

  @override
  Widget build(BuildContext context) => IgnorePointer(
        child: Opacity(
          opacity: .72,
          child: SvgPicture.asset(
            'assets/sidebar_mountains.svg',
            fit: BoxFit.cover,
            alignment: Alignment.bottomCenter,
          ),
        ),
      );
}

class _Central17UsMap extends StatefulWidget {
  const _Central17UsMap({required this.states, required this.partners});
  final List<Map<String, dynamic>> states;
  final List<Map<String, dynamic>> partners;

  @override
  State<_Central17UsMap> createState() => _Central17UsMapState();
}

class _Central17UsMapState extends State<_Central17UsMap> {
  Future<String>? _svgFuture;

  static const _stateCodes = <String, String>{
    'Alabama':'AL','Arizona':'AZ','Arkansas':'AR','California':'CA','Colorado':'CO','Connecticut':'CT',
    'Delaware':'DE','Florida':'FL','Georgia':'GA','Idaho':'ID','Illinois':'IL','Indiana':'IN','Iowa':'IA',
    'Kansas':'KS','Kentucky':'KY','Louisiana':'LA','Maine':'ME','Maryland':'MD','Massachusetts':'MA',
    'Michigan':'MI','Minnesota':'MN','Mississippi':'MS','Missouri':'MO','Montana':'MT','Nebraska':'NE',
    'Nevada':'NV','New Hampshire':'NH','New Jersey':'NJ','New Mexico':'NM','New York':'NY',
    'North Carolina':'NC','North Dakota':'ND','Ohio':'OH','Oklahoma':'OK','Oregon':'OR','Pennsylvania':'PA',
    'Rhode Island':'RI','South Carolina':'SC','South Dakota':'SD','Tennessee':'TN','Texas':'TX','Utah':'UT',
    'Vermont':'VT','Virginia':'VA','Washington':'WA','West Virginia':'WV','Wisconsin':'WI','Wyoming':'WY',
    'District of Columbia':'DC','Alaska':'AK','Hawaii':'HI',
  };

  // Approximate state centroids normalized to the bundled SVG. They are used
  // for active pins and for resolving a click on the map to the nearest state.
  static const _positions = <String, Offset>{
    'Alabama':Offset(.67,.67),'Alaska':Offset(.16,.87),'Arizona':Offset(.25,.60),'Arkansas':Offset(.55,.61),
    'California':Offset(.105,.52),'Colorado':Offset(.37,.43),'Connecticut':Offset(.91,.35),'Delaware':Offset(.86,.46),
    'District of Columbia':Offset(.84,.48),'Florida':Offset(.80,.79),'Georgia':Offset(.73,.68),'Hawaii':Offset(.31,.88),
    'Idaho':Offset(.22,.30),'Illinois':Offset(.63,.44),'Indiana':Offset(.68,.45),'Iowa':Offset(.55,.39),
    'Kansas':Offset(.47,.50),'Kentucky':Offset(.68,.52),'Louisiana':Offset(.56,.72),'Maine':Offset(.94,.19),
    'Maryland':Offset(.83,.47),'Massachusetts':Offset(.92,.32),'Michigan':Offset(.69,.29),'Minnesota':Offset(.54,.27),
    'Mississippi':Offset(.62,.67),'Missouri':Offset(.57,.51),'Montana':Offset(.34,.23),'Nebraska':Offset(.45,.41),
    'Nevada':Offset(.18,.43),'New Hampshire':Offset(.91,.28),'New Jersey':Offset(.86,.40),'New Mexico':Offset(.34,.60),
    'New York':Offset(.86,.30),'North Carolina':Offset(.79,.56),'North Dakota':Offset(.45,.25),'Ohio':Offset(.73,.43),
    'Oklahoma':Offset(.48,.59),'Oregon':Offset(.10,.28),'Pennsylvania':Offset(.81,.39),'Rhode Island':Offset(.93,.36),
    'South Carolina':Offset(.76,.62),'South Dakota':Offset(.45,.33),'Tennessee':Offset(.66,.58),'Texas':Offset(.48,.73),
    'Utah':Offset(.28,.46),'Vermont':Offset(.89,.27),'Virginia':Offset(.81,.49),'Washington':Offset(.10,.16),
    'West Virginia':Offset(.77,.48),'Wisconsin':Offset(.62,.32),'Wyoming':Offset(.34,.34),
  };

  @override
  void initState() {
    super.initState();
    _svgFuture = _styledSvg();
  }

  @override
  void didUpdateWidget(covariant _Central17UsMap oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.states.toString() != widget.states.toString()) {
      _svgFuture = _styledSvg();
    }
  }

  Future<String> _styledSvg() async {
    var raw = await rootBundle.loadString('assets/us_states_map.svg');
    final active = <String>{};
    for (final row in widget.states) {
      if ((row['count'] as num?)?.toInt() == 0) continue;
      final code = _stateCodes['${row['state'] ?? ''}'];
      if (code != null) active.add(code);
    }
    for (final code in active) {
      raw = raw.replaceFirst('<g id="$code">', '<g id="$code" fill="#173B66">');
    }
    return raw;
  }

  List<Map<String, dynamic>> _partnersFor(String state) => widget.partners
      .where((p) =>
          '${p['state'] ?? ''}' == state &&
          '${p['lifecycle'] ?? ''}'.toUpperCase() == 'LIVE')
      .map((p) => Map<String, dynamic>.from(p))
      .toList();

  String? _nearestState(Offset point, Size size) {
    if (size.width <= 0 || size.height <= 0) return null;
    final normalized = Offset(point.dx / size.width, point.dy / size.height);
    String? best;
    var bestDistance = double.infinity;
    for (final entry in _positions.entries) {
      final dx = normalized.dx - entry.value.dx;
      final dy = normalized.dy - entry.value.dy;
      // Horizontal distance matters slightly more on the very wide US map.
      final distance = dx * dx + dy * dy * .72;
      if (distance < bestDistance) {
        bestDistance = distance;
        best = entry.key;
      }
    }
    return best;
  }

  void _openState(String state) {
    final partners = _partnersFor(state);
    showDialog<void>(
      context: context,
      builder: (dialogContext) => Dialog(
        backgroundColor: brandWhite,
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(16)),
        child: ConstrainedBox(
          constraints: const BoxConstraints(maxWidth: 420),
          child: Padding(
            padding: const EdgeInsets.all(20),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(children:[
                  Container(width:38,height:38,decoration:BoxDecoration(color:brandGold.withOpacity(.12),borderRadius:BorderRadius.circular(11)),child:const Icon(Icons.location_on_rounded,color:brandGold,size:21)),
                  const SizedBox(width:10),
                  Expanded(child:LText(state,style:GoogleFonts.lora(color:brandNavy,fontSize:23,fontWeight:FontWeight.w700))),
                  IconButton(onPressed:()=>Navigator.pop(dialogContext),icon:const Icon(Icons.close_rounded)),
                ]),
                const SizedBox(height:8),
                LText('${partners.length} ${uiLiteral(partners.length == 1 ? 'active partner' : 'active partners')}',style:const TextStyle(color:brandTextSoft,fontSize:11)),
                const SizedBox(height:12),
                if (partners.isEmpty)
                  _MessageCard(
                    icon: Icons.location_off_outlined,
                    title: uiLiteral('No active partners in this state'),
                    message: uiBilingual(
                      'There are currently 0 active HIMATE partners in $state.',
                      'Jelenleg 0 aktív HIMATE partner van ebben az államban: $state.',
                    ),
                  )
                else
                  ConstrainedBox(
                    constraints: const BoxConstraints(maxHeight: 360),
                    child: SingleChildScrollView(
                      child: Column(
                        children: [
                          for (var i=0;i<partners.length;i++) ...[
                            Container(
                              width:double.infinity,
                              padding:const EdgeInsets.symmetric(horizontal:12,vertical:11),
                              decoration:BoxDecoration(color:const Color(0xFFF8FAFD),borderRadius:BorderRadius.circular(11),border:Border.all(color:brandMist)),
                              child:Row(children:[
                                const Icon(Icons.apartment_rounded,color:brandNavy,size:18),
                                const SizedBox(width:9),
                                Expanded(child:Column(crossAxisAlignment:CrossAxisAlignment.start,children:[
                                  LText('${partners[i]['name'] ?? '—'}',style:const TextStyle(color:brandNavy,fontSize:11.5,fontWeight:FontWeight.w700)),
                                  const SizedBox(height:2),
                                  LText('${partners[i]['city'] ?? ''}${('${partners[i]['city'] ?? ''}').isNotEmpty ? ', ' : ''}$state',style:const TextStyle(color:brandTextSoft,fontSize:9.5)),
                                ])),
                                _StatusPill(label:'${partners[i]['lifecycle'] ?? 'LIVE'}'),
                              ]),
                            ),
                            if (i != partners.length - 1) const SizedBox(height:7),
                          ],
                        ],
                      ),
                    ),
                  ),
              ],
            ),
          ),
        ),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final activeRows = widget.states.where((r)=>(r['count'] as num?)?.toInt() != 0).toList()
      ..sort((a,b)=>((b['count'] as num?)?.toInt() ?? 0).compareTo((a['count'] as num?)?.toInt() ?? 0));
    final labelledStates = activeRows.take(5).map((e)=>'${e['state'] ?? ''}').toSet();
    return FutureBuilder<String>(
      future: _svgFuture,
      builder: (context, snapshot) {
        if (!snapshot.hasData) return const Center(child:SizedBox(width:22,height:22,child:CircularProgressIndicator(strokeWidth:2)));
        return LayoutBuilder(
          builder:(context,c)=>MouseRegion(
            cursor:SystemMouseCursors.click,
            child:GestureDetector(
              behavior:HitTestBehavior.opaque,
              onTapUp:(details){
                final state=_nearestState(details.localPosition,Size(c.maxWidth,c.maxHeight));
                if(state!=null)_openState(state);
              },
              child:Stack(
                clipBehavior:Clip.none,
                children:[
                  Positioned.fill(child:SvgPicture.string(snapshot.data!,fit:BoxFit.contain,alignment:Alignment.center)),
              for(final row in activeRows)
                if(_positions['${row['state'] ?? ''}'] case final Offset pos)
                  Positioned(
                    left:c.maxWidth*pos.dx-8,
                    top:c.maxHeight*pos.dy-15,
                    child:Tooltip(
                      message:'${row['state']} · ${row['count']}',
                      child:InkWell(
                        onTap:()=>_openState('${row['state']}'),
                        borderRadius:BorderRadius.circular(99),
                        child:const Padding(
                          padding:EdgeInsets.all(4),
                          child:Icon(Icons.location_on_rounded,color:brandGold,size:24,shadows:[Shadow(color:Colors.white,blurRadius:5)]),
                        ),
                      ),
                    ),
                  ),
              for(final row in activeRows)
                if(labelledStates.contains('${row['state'] ?? ''}'))
                  if(_positions['${row['state'] ?? ''}'] case final Offset pos)
                  Positioned(
                    left:(c.maxWidth*pos.dx).clamp(8.0, math.max(8.0, c.maxWidth-98)).toDouble(),
                    top:(c.maxHeight*pos.dy-34).clamp(4.0, math.max(4.0, c.maxHeight-34)).toDouble(),
                    child:IgnorePointer(
                      child:Container(
                        padding:const EdgeInsets.symmetric(horizontal:6,vertical:4),
                        decoration:BoxDecoration(color:brandWhite.withOpacity(.94),borderRadius:BorderRadius.circular(7),boxShadow:[BoxShadow(color:brandNavy.withOpacity(.08),blurRadius:8,offset:const Offset(0,2))]),
                        child:Column(crossAxisAlignment:CrossAxisAlignment.start,children:[
                          LText('${row['state']}',style:const TextStyle(color:brandNavy,fontSize:8.5,fontWeight:FontWeight.w700)),
                          LText('${row['count']} ${uiLiteral('partners')}',style:const TextStyle(color:brandTextSoft,fontSize:7.6)),
                        ]),
                      ),
                    ),
                  ),
                ],
              ),
            ),
          ),
        );
      },
    );
  }
}

class _Central17TrendChart extends StatelessWidget {
  const _Central17TrendChart({
    required this.partnerValues,
    required this.moneyValues,
    required this.labels,
  });
  final List<double> partnerValues;
  final List<double> moneyValues;
  final List<String> labels;

  @override
  Widget build(BuildContext context) => CustomPaint(
        painter: _Central17TrendPainter(partnerValues, moneyValues, labels),
        child: const SizedBox.expand(),
      );
}

class _Central17TrendPainter extends CustomPainter {
  _Central17TrendPainter(this.partnerValues, this.moneyValues, this.labels);
  final List<double> partnerValues;
  final List<double> moneyValues;
  final List<String> labels;

  @override
  void paint(Canvas canvas, Size size) {
    const left=46.0,right=48.0,top=8.0,bottom=28.0;
    final rect=Rect.fromLTWH(left,top,size.width-left-right,size.height-top-bottom);
    final grid=Paint()..color=const Color(0xFFE5EAF1)..strokeWidth=.8;
    for(var i=0;i<=4;i++){
      final y=rect.top+rect.height*i/4;
      canvas.drawLine(Offset(rect.left,y),Offset(rect.right,y),grid);
    }
    final n=math.max(partnerValues.length,moneyValues.length);
    if(n==0)return;
    double maxPartners=1,maxMoney=1;
    for(final v in partnerValues){if(v>maxPartners)maxPartners=v;}
    for(final v in moneyValues){if(v>maxMoney)maxMoney=v;}
    final slot=rect.width/n;
    final barPaint=Paint()..color=const Color(0xFFBFD5F3).withOpacity(.82);
    for(var i=0;i<partnerValues.length;i++){
      final h=rect.height*(partnerValues[i]/maxPartners);
      final bar=RRect.fromRectAndRadius(Rect.fromLTWH(rect.left+i*slot+slot*.18,rect.bottom-h,slot*.58,h),const Radius.circular(3));
      canvas.drawRRect(bar,barPaint);
    }
    if(moneyValues.isNotEmpty){
      final path=Path();
      for(var i=0;i<moneyValues.length;i++){
        final x=rect.left+i*slot+slot*.47;
        final y=rect.bottom-rect.height*(moneyValues[i]/maxMoney);
        if(i==0)path.moveTo(x,y);else path.lineTo(x,y);
      }
      canvas.drawPath(path,Paint()..color=brandGold..strokeWidth=2.1..style=PaintingStyle.stroke..strokeCap=StrokeCap.round..strokeJoin=StrokeJoin.round);
      for(var i=0;i<moneyValues.length;i++){
        final x=rect.left+i*slot+slot*.47;
        final y=rect.bottom-rect.height*(moneyValues[i]/maxMoney);
        canvas.drawCircle(Offset(x,y),3.1,Paint()..color=brandGold);
      }
    }
    final count=math.min(labels.length,n);
    for(var i=0;i<count;i++){
      final tp=TextPainter(text:TextSpan(text:labels[i],style:GoogleFonts.inter(fontSize:8.3,color:brandTextSoft)),textDirection:TextDirection.ltr)..layout();
      tp.paint(canvas,Offset(rect.left+i*slot+slot*.47-tp.width/2,rect.bottom+8));
    }
    for(var i=0;i<=4;i++){
      final p=maxPartners*(4-i)/4;
      final tp=TextPainter(text:TextSpan(text:intl.NumberFormat.compact().format(p),style:GoogleFonts.inter(fontSize:7.8,color:brandTextSoft)),textDirection:TextDirection.ltr)..layout();
      tp.paint(canvas,Offset(rect.left-tp.width-7,rect.top+rect.height*i/4-tp.height/2));
      final m=maxMoney*(4-i)/4;
      final mt=TextPainter(text:TextSpan(text:intl.NumberFormat.compact().format(m),style:GoogleFonts.inter(fontSize:7.8,color:brandTextSoft)),textDirection:TextDirection.ltr)..layout();
      mt.paint(canvas,Offset(rect.right+7,rect.top+rect.height*i/4-mt.height/2));
    }
  }

  @override
  bool shouldRepaint(covariant _Central17TrendPainter oldDelegate) =>
      oldDelegate.partnerValues.toString()!=partnerValues.toString() ||
      oldDelegate.moneyValues.toString()!=moneyValues.toString() ||
      oldDelegate.labels.toString()!=labels.toString();
}


class _Central17TrendCard extends StatelessWidget {
  const _Central17TrendCard({
    required this.partnerTrend,
    required this.revenueTrend,
    required this.currency,
    required this.year,
  });
  final List<Map<String,dynamic>> partnerTrend;
  final List<Map<String,dynamic>> revenueTrend;
  final String currency;
  final int year;

  @override
  Widget build(BuildContext context) {
    final locale=himateLocaleCode(Localizations.localeOf(context));
    final partnerByMonth=<int,double>{};
    for(final row in partnerTrend){
      final m=(row['month'] as num?)?.toInt() ?? 0;
      if(m>=1&&m<=12) partnerByMonth[m]=number(row['value']);
    }
    final revenueByMonth=<int,double>{};
    for(final row in revenueTrend){
      final m=(row['month'] as num?)?.toInt() ?? 0;
      if(m<1||m>12)continue;
      if(currency.isNotEmpty && '${row['currency'] ?? ''}'!=currency)continue;
      revenueByMonth[m]=number(row['revenue']);
    }
    final partners=List<double>.generate(12,(i)=>partnerByMonth[i+1] ?? 0);
    final revenue=List<double>.generate(12,(i)=>revenueByMonth[i+1] ?? 0);
    final labels=List<String>.generate(12,(i)=>intl.DateFormat.MMM(locale).format(DateTime(year,i+1)).replaceAll('.', ''));

    return SizedBox(
      height:330,
      child:Card(
        child:Padding(
          padding:const EdgeInsets.fromLTRB(20,17,18,16),
          child:Column(crossAxisAlignment:CrossAxisAlignment.start,children:[
            Row(children:[
              Container(width:34,height:34,decoration:BoxDecoration(color:brandSteel.withOpacity(.08),borderRadius:BorderRadius.circular(10)),child:const Icon(Icons.bar_chart_rounded,color:brandSteel,size:20)),
              const SizedBox(width:10),
              Expanded(child:Column(crossAxisAlignment:CrossAxisAlignment.start,children:[
                LText(uiLiteral('12 month trend'),style:GoogleFonts.lora(color:brandNavy,fontSize:20,fontWeight:FontWeight.w700)),
                LText(uiLiteral('Partner count and settled amounts'),style:const TextStyle(color:brandTextSoft,fontSize:9.5)),
              ])),
              _Central17LegendDot(color:brandNavy,label:uiLiteral('Active partners')),
              const SizedBox(width:14),
              _Central17LegendDot(color:brandGold,label:currency.isEmpty?uiLiteral('Settled amount'): '${uiLiteral('Settled amount')} ($currency)'),
            ]),
            const SizedBox(height:10),
            Expanded(child:_Central17TrendChart(partnerValues:partners,moneyValues:revenue,labels:labels)),
          ]),
        ),
      ),
    );
  }
}

class _Central17LegendDot extends StatelessWidget {
  const _Central17LegendDot({required this.color,required this.label});
  final Color color;
  final String label;
  @override
  Widget build(BuildContext context)=>Row(mainAxisSize:MainAxisSize.min,children:[
    Container(width:7,height:7,decoration:BoxDecoration(color:color,shape:BoxShape.circle)),
    const SizedBox(width:5),
    LText(label,style:const TextStyle(color:brandTextSoft,fontSize:8.7,fontWeight:FontWeight.w600)),
  ]);
}


class _Central17ModuleTrendCard extends StatelessWidget {
  const _Central17ModuleTrendCard({required this.trend});
  final List<Map<String,dynamic>> trend;

  @override
  Widget build(BuildContext context) {
    final year=DateTime.now().toUtc().year;
    final locale=himateLocaleCode(Localizations.localeOf(context));
    final modules=List<double>.filled(12,0);
    final partners=List<double>.filled(12,0);
    for(final row in trend){
      final month=(row['month'] as num?)?.toInt() ?? 0;
      if(month<1||month>12)continue;
      modules[month-1]=number(row['active_modules']);
      partners[month-1]=number(row['active_partners']);
    }
    final labels=List<String>.generate(12,(i)=>intl.DateFormat.MMM(locale).format(DateTime(year,i+1)).replaceAll('.', ''));
    return SizedBox(
      height:250,
      child:Card(
        child:Padding(
          padding:const EdgeInsets.fromLTRB(20,16,18,14),
          child:Column(crossAxisAlignment:CrossAxisAlignment.start,children:[
            Row(children:[
              Container(width:34,height:34,decoration:BoxDecoration(color:brandSteel.withOpacity(.08),borderRadius:BorderRadius.circular(10)),child:const Icon(Icons.bar_chart_rounded,color:brandSteel,size:20)),
              const SizedBox(width:10),
              Expanded(child:Column(crossAxisAlignment:CrossAxisAlignment.start,children:[
                LText(uiLiteral('Module usage growth'),style:GoogleFonts.lora(color:brandNavy,fontSize:20,fontWeight:FontWeight.w700)),
                LText(uiLiteral('Active module count by month'),style:const TextStyle(color:brandTextSoft,fontSize:9.5)),
              ])),
              _Central17LegendDot(color:brandSteel,label:uiLiteral('Active modules')),
              const SizedBox(width:14),
              _Central17LegendDot(color:brandGold,label:uiLiteral('Active partners')),
            ]),
            const SizedBox(height:8),
            Expanded(
              child: Stack(
                alignment: Alignment.center,
                children: [
                  _Central17TrendChart(partnerValues:modules,moneyValues:partners,labels:labels),
                  if (trend.isEmpty)
                    IgnorePointer(
                      child: Container(
                        padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 7),
                        decoration: BoxDecoration(
                          color: brandWhite.withOpacity(.88),
                          borderRadius: BorderRadius.circular(8),
                        ),
                        child: LText(
                          uiLiteral('No module usage history is available yet.'),
                          style: const TextStyle(color:brandTextSoft,fontSize:9.5),
                        ),
                      ),
                    ),
                ],
              ),
            ),
          ]),
        ),
      ),
    );
  }
}


class _Central17SoftChip extends StatelessWidget {
  const _Central17SoftChip({required this.label});
  final String label;

  @override
  Widget build(BuildContext context) => Container(
        padding: const EdgeInsets.symmetric(horizontal: 9, vertical: 5),
        decoration: BoxDecoration(
          color: const Color(0xFFF1F4F8),
          borderRadius: BorderRadius.circular(99),
        ),
        child: LText(label, style: const TextStyle(color: brandTextSoft, fontSize: 8.5, fontWeight: FontWeight.w600)),
      );
}
